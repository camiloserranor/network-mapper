package gnmi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// Client wraps a gNMI connection to a single switch.
type Client struct {
	address  string
	conn     *grpc.ClientConn
	gnmi     gpb.GNMIClient
	username string
	password string
	encoding gpb.Encoding
}

// Notification holds a decoded gNMI notification.
type Notification struct {
	Timestamp int64
	Updates   []Update
}

// Update holds a single path-value pair from a gNMI response.
type Update struct {
	Path  string
	Value interface{}
}

// ClientOptions configures a gNMI client connection.
type ClientOptions struct {
	Address    string
	Username   string
	Password   string
	TLS        TLSOptions
	Encoding   string // "JSON_IETF" or "JSON"
	TimeoutSec int
}

// NewClient creates a gNMI client connected to the given switch.
func NewClient(ctx context.Context, opts ClientOptions) (*Client, error) {
	dialOpts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(64 * 1024 * 1024)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// TLS configuration
	if opts.TLS.SkipVerify || opts.TLS.TOFU || opts.TLS.CACert != "" || opts.TLS.ClientCert != "" {
		tlsCfg, err := BuildTLSConfig(opts.Address, opts.TLS)
		if err != nil {
			return nil, fmt.Errorf("TLS setup for %s: %w", opts.Address, err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, opts.Address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("gRPC dial %s: %w", opts.Address, err)
	}

	encoding := gpb.Encoding_JSON_IETF
	if strings.EqualFold(opts.Encoding, "JSON") {
		encoding = gpb.Encoding_JSON
	}

	return &Client{
		address:  opts.Address,
		conn:     conn,
		gnmi:     gpb.NewGNMIClient(conn),
		username: opts.Username,
		password: opts.Password,
		encoding: encoding,
	}, nil
}

// Close releases the gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CapabilityResult holds the result of a gNMI Capabilities request.
type CapabilityResult struct {
	GNMIVersion    string
	Models         []ModelData
	Encodings      []string
}

// ModelData describes a single supported YANG model.
type ModelData struct {
	Name         string
	Organization string
	Version      string
}

// Capabilities sends a gNMI Capabilities RPC to discover what the switch supports.
func (c *Client) Capabilities(ctx context.Context) (*CapabilityResult, error) {
	ctx = c.authContext(ctx)

	resp, err := c.gnmi.Capabilities(ctx, &gpb.CapabilityRequest{})
	if err != nil {
		return nil, fmt.Errorf("gNMI Capabilities on %s: %w", c.address, err)
	}

	result := &CapabilityResult{
		GNMIVersion: resp.GetGNMIVersion(),
	}

	for _, m := range resp.GetSupportedModels() {
		result.Models = append(result.Models, ModelData{
			Name:         m.GetName(),
			Organization: m.GetOrganization(),
			Version:      m.GetVersion(),
		})
	}

	for _, e := range resp.GetSupportedEncodings() {
		result.Encodings = append(result.Encodings, e.String())
	}

	return result, nil
}

// Get performs a gNMI Get request for the given YANG path.
func (c *Client) Get(ctx context.Context, yangPath string) ([]Notification, error) {
	ctx = c.authContext(ctx)

	path, err := parsePath(yangPath)
	if err != nil {
		return nil, fmt.Errorf("parsing path %q: %w", yangPath, err)
	}

	req := &gpb.GetRequest{
		Path:     []*gpb.Path{path},
		Type:     gpb.GetRequest_STATE,
		Encoding: c.encoding,
	}

	resp, err := c.gnmi.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gNMI Get %q on %s: %w", yangPath, c.address, err)
	}

	return decodeGetResponse(resp, c.encoding)
}

// SubscribeOnce performs a Subscribe ONCE request (for list paths where Get returns empty on SONiC).
func (c *Client) SubscribeOnce(ctx context.Context, yangPath string) ([]Notification, error) {
	ctx = c.authContext(ctx)

	path, err := parsePath(yangPath)
	if err != nil {
		return nil, fmt.Errorf("parsing path %q: %w", yangPath, err)
	}

	req := &gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Subscription: []*gpb.Subscription{
					{Path: path, Mode: gpb.SubscriptionMode_TARGET_DEFINED},
				},
				Mode:     gpb.SubscriptionList_ONCE,
				Encoding: c.encoding,
			},
		},
	}

	stream, err := c.gnmi.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("subscribe stream to %s: %w", c.address, err)
	}
	if err := stream.Send(req); err != nil {
		return nil, fmt.Errorf("sending subscribe request: %w", err)
	}

	var allNotifs []Notification
	for {
		resp, err := stream.Recv()
		if err != nil {
			return nil, fmt.Errorf("subscribe recv: %w", err)
		}

		switch r := resp.Response.(type) {
		case *gpb.SubscribeResponse_Update:
			notif := decodeSubscribeNotification(r.Update, c.encoding)
			if notif != nil {
				allNotifs = append(allNotifs, *notif)
			}
		case *gpb.SubscribeResponse_SyncResponse:
			return allNotifs, nil
		}

		if len(allNotifs) > 1000 {
			log.Printf("WARN: subscribe-once cap reached for %s", c.address)
			return allNotifs, nil
		}
	}
}

// GetWithFallback tries Get first; if the result is empty, falls back to SubscribeOnce.
// This handles the SONiC quirk where bulk Get on list paths returns empty.
func (c *Client) GetWithFallback(ctx context.Context, yangPath string) ([]Notification, error) {
	notifs, err := c.Get(ctx, yangPath)
	if err != nil {
		return nil, err
	}

	// Check if we got meaningful data
	hasData := false
	for _, n := range notifs {
		if len(n.Updates) > 0 {
			hasData = true
			break
		}
	}

	if hasData {
		return notifs, nil
	}

	log.Printf("Get returned empty for %s on %s, falling back to SubscribeOnce", yangPath, c.address)
	return c.SubscribeOnce(ctx, yangPath)
}

func (c *Client) authContext(ctx context.Context) context.Context {
	if c.username != "" || c.password != "" {
		md := metadata.Pairs("username", c.username, "password", c.password)
		return metadata.NewOutgoingContext(ctx, md)
	}
	return ctx
}

// parsePath converts a YANG path string to a gNMI Path proto.
// e.g., "/openconfig-lldp:lldp/interfaces/interface/neighbors"
func parsePath(yangPath string) (*gpb.Path, error) {
	yangPath = strings.TrimPrefix(yangPath, "/")
	if yangPath == "" {
		return &gpb.Path{}, nil
	}

	parts := strings.Split(yangPath, "/")
	elems := make([]*gpb.PathElem, 0, len(parts))

	for _, part := range parts {
		// Handle key selectors: interface[name=eth0]
		if idx := strings.Index(part, "["); idx != -1 {
			name := part[:idx]
			keyStr := part[idx+1 : len(part)-1]
			keys := make(map[string]string)
			for _, kv := range strings.Split(keyStr, ",") {
				eqIdx := strings.Index(kv, "=")
				if eqIdx > 0 {
					keys[kv[:eqIdx]] = kv[eqIdx+1:]
				}
			}
			elems = append(elems, &gpb.PathElem{Name: name, Key: keys})
		} else {
			// Strip module prefix: "openconfig-lldp:lldp" → "lldp"
			name := part
			if colonIdx := strings.Index(part, ":"); colonIdx != -1 {
				name = part[colonIdx+1:]
			}
			elems = append(elems, &gpb.PathElem{Name: name})
		}
	}

	return &gpb.Path{Elem: elems}, nil
}

func decodeGetResponse(resp *gpb.GetResponse, encoding gpb.Encoding) ([]Notification, error) {
	var notifs []Notification
	for _, notif := range resp.GetNotification() {
		n := Notification{Timestamp: notif.GetTimestamp()}
		for _, upd := range notif.GetUpdate() {
			pathStr := pathToString(upd.GetPath())
			val, err := decodeTypedValue(upd.GetVal(), encoding)
			if err != nil {
				log.Printf("WARN: decode error at %s: %v", pathStr, err)
				continue
			}
			n.Updates = append(n.Updates, Update{Path: pathStr, Value: val})
		}
		if len(n.Updates) > 0 {
			notifs = append(notifs, n)
		}
	}
	return notifs, nil
}

func decodeSubscribeNotification(notif *gpb.Notification, encoding gpb.Encoding) *Notification {
	n := &Notification{Timestamp: notif.GetTimestamp()}
	for _, upd := range notif.GetUpdate() {
		pathStr := pathToString(upd.GetPath())
		val, err := decodeTypedValue(upd.GetVal(), encoding)
		if err != nil {
			continue
		}
		n.Updates = append(n.Updates, Update{Path: pathStr, Value: val})
	}
	if len(n.Updates) == 0 {
		return nil
	}
	return n
}

func decodeTypedValue(val *gpb.TypedValue, encoding gpb.Encoding) (interface{}, error) {
	if val == nil {
		return nil, fmt.Errorf("nil typed value")
	}

	switch v := val.GetValue().(type) {
	case *gpb.TypedValue_JsonVal:
		var result interface{}
		if err := json.Unmarshal(v.JsonVal, &result); err != nil {
			return nil, err
		}
		return result, nil

	case *gpb.TypedValue_JsonIetfVal:
		var result interface{}
		if err := json.Unmarshal(v.JsonIetfVal, &result); err != nil {
			return nil, err
		}
		// Strip module prefixes from keys (RFC 7951)
		result = stripModulePrefixes(result)
		// Unwrap single-key wrapper map
		if m, ok := result.(map[string]interface{}); ok && len(m) == 1 {
			for _, inner := range m {
				return inner, nil
			}
		}
		return result, nil

	case *gpb.TypedValue_StringVal:
		return v.StringVal, nil
	case *gpb.TypedValue_IntVal:
		return v.IntVal, nil
	case *gpb.TypedValue_UintVal:
		return v.UintVal, nil
	case *gpb.TypedValue_BoolVal:
		return v.BoolVal, nil
	case *gpb.TypedValue_FloatVal:
		return v.FloatVal, nil
	}

	return nil, fmt.Errorf("unsupported TypedValue type: %T", val.GetValue())
}

// stripModulePrefixes recursively removes "module-name:" prefixes from JSON_IETF map keys.
func stripModulePrefixes(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		stripped := make(map[string]interface{}, len(val))
		for k, child := range val {
			newKey := k
			if idx := strings.Index(k, ":"); idx != -1 {
				newKey = k[idx+1:]
			}
			stripped[newKey] = stripModulePrefixes(child)
		}
		return stripped
	case []interface{}:
		for i, item := range val {
			val[i] = stripModulePrefixes(item)
		}
		return val
	default:
		return v
	}
}

func pathToString(path *gpb.Path) string {
	if path == nil {
		return "/"
	}
	var parts []string
	for _, elem := range path.GetElem() {
		s := elem.GetName()
		if len(elem.GetKey()) > 0 {
			var keys []string
			for k, v := range elem.GetKey() {
				keys = append(keys, k+"="+v)
			}
			s += "[" + strings.Join(keys, ",") + "]"
		}
		parts = append(parts, s)
	}
	return "/" + strings.Join(parts, "/")
}
