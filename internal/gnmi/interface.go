package gnmi

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GNMIClient defines the interface for gNMI operations used by platform
// collectors. This allows testing with mock implementations.
type GNMIClient interface {
	Get(ctx context.Context, yangPath string) ([]Notification, error)
	GetWithFallback(ctx context.Context, yangPath string) ([]Notification, error)
	SubscribeOnce(ctx context.Context, yangPath string) ([]Notification, error)
	Capabilities(ctx context.Context) (*CapabilityResult, error)
	Close() error
}

// ErrPathNotSupported indicates the switch does not support the requested path.
type ErrPathNotSupported struct {
	Path    string
	Message string
}

func (e *ErrPathNotSupported) Error() string {
	return fmt.Sprintf("path not supported: %s: %s", e.Path, e.Message)
}

// ErrAuth indicates an authentication or authorization failure.
type ErrAuth struct {
	Message string
}

func (e *ErrAuth) Error() string {
	return fmt.Sprintf("auth error: %s", e.Message)
}

// IsPathNotSupported returns true if the error indicates the switch does not
// support the requested gNMI path (InvalidArgument with path-related messages).
func IsPathNotSupported(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	if st.Code() != codes.InvalidArgument {
		return false
	}
	msg := strings.ToLower(st.Message())
	return strings.Contains(msg, "path") ||
		strings.Contains(msg, "namespace") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "not supported")
}
