// Package output provides implementations for output modules.
// Output modules are responsible for sending data to destination systems.
//
// This package implements Epic 3: Module Execution - Output modules.
package output

import (
	"context"

	"github.com/cannectors/runtime/pkg/connector"
)

// Module represents an output module that sends data to a destination system.
//
// # Responsibilities
//
// Output modules are responsible for:
//   - Sending data to external destination systems (APIs, databases, message queues, etc.)
//   - Formatting data appropriately for the destination (JSON, XML, etc.)
//   - Handling destination-specific protocols and authentication
//   - Managing resources (connections, file handles, etc.) and cleaning them up via Close()
//
// # What Output Modules Should NOT Do
//
// Output modules should NOT:
//   - Fetch data from sources (that's the input module's responsibility)
//   - Transform or filter data (that's the filter module's responsibility)
//   - Perform business logic beyond data transmission
//   - Store state between Send() calls (modules should be stateless or manage their own state)
//
// # Context Usage
//
// The context.Context parameter in Send() should be used for:
//   - Cancellation: Respect ctx.Done() to cancel long-running operations
//   - Timeouts: Use context timeouts to limit operation duration
//   - Request-scoped values: Access request metadata if needed
//
// Do NOT use context for storing module state or configuration.
//
// # Error Handling
//
// Return errors when:
//   - Network/connection failures occur
//   - Authentication fails
//   - Data cannot be sent (destination unavailable, invalid format)
//   - Configuration is invalid
//
// The runtime will handle errors according to pipeline error handling configuration.
// Do NOT retry internally unless explicitly required by the module's design.
//
// # Return Format
//
// Send() must return:
//   - The number of records successfully sent (int)
//   - An error if sending fails (nil if all records were sent successfully)
//
// If partial success occurs (some records sent, some failed), implementations should:
//   - Return the count of successfully sent records
//   - Return an error describing the failure
//   - The runtime will track this as a partial success scenario
//
// Example:
//
//	sentCount, err := module.Send(ctx, records)
//	// sentCount = 5, err = nil (all 5 records sent successfully)
//	// sentCount = 3, err = ErrPartialFailure (3 sent, 2 failed)
//
// # Record Format
//
// Send() receives []map[string]any where:
//   - Each map represents a single record/entity
//   - Keys are field names (strings)
//   - Values can be any JSON-serializable type
//
// Output modules are responsible for serializing this data appropriately for the destination.
//
// # Resource Management
//
// Close() must release all resources held by the module:
//   - Close HTTP connections and connection pools
//   - Close file handles
//   - Release database connections
//   - Clean up any temporary resources
//
// Close() may be called multiple times and should be idempotent.
// The runtime will call Close() at the end of pipeline execution.
//
// # Stability
//
// This interface is designed to remain stable across versions.
// New methods will not be added to this interface to maintain backward compatibility.
// Optional capabilities (like preview) are provided via separate interfaces (e.g., PreviewableModule).
//
// # Example Implementation
//
// Here's a complete example of implementing an output module:
//
//	type MyOutputModule struct {
//	    client   *http.Client
//	    endpoint string
//	}
//
//	func (m *MyOutputModule) Send(ctx context.Context, records []map[string]any) (int, error) {
//	    // Respect context cancellation
//	    select {
//	    case <-ctx.Done():
//	        return 0, ctx.Err()
//	    default:
//	    }
//
//	    // Serialize records
//	    body, err := json.Marshal(records)
//	    if err != nil {
//	        return 0, err
//	    }
//
//	    // Send to destination
//	    req, err := http.NewRequestWithContext(ctx, "POST", m.endpoint, bytes.NewReader(body))
//	    if err != nil {
//	        return 0, err
//	    }
//	    req.Header.Set("Content-Type", "application/json")
//
//	    resp, err := m.client.Do(req)
//	    if err != nil {
//	        return 0, err
//	    }
//	    defer resp.Body.Close()
//
//	    if resp.StatusCode >= 400 {
//	        return 0, fmt.Errorf("request failed with status %d", resp.StatusCode)
//	    }
//
//	    return len(records), nil
//	}
//
//	func (m *MyOutputModule) Close() error {
//	    m.client.CloseIdleConnections()
//	    return nil
//	}
//
//	// Verify interface compliance at compile time
//	var _ Module = (*MyOutputModule)(nil)
type Module interface {
	// Send transmits records to the destination system.
	//
	// The context can be used to cancel long-running operations.
	// Implementations should respect ctx.Done() and return promptly when canceled.
	//
	// The records parameter contains the data to send.
	// Returns the number of records successfully sent and any error.
	// If partial success occurs, return the count of successful sends and an error.
	Send(ctx context.Context, records []map[string]any) (int, error)

	// Close releases any resources held by the module.
	//
	// This method should be idempotent (safe to call multiple times).
	// The runtime will call Close() at the end of pipeline execution.
	// Implementations must release all resources (connections, file handles, etc.).
	Close() error
}

// ConnectableModule is implemented by output modules that hold a connection to
// their destination and can open it on demand.
//
// The runtime opens it before the input stage of a real run, so a destination
// that cannot be reached fails before the pipeline has fetched anything or
// enriched anything. In dry-run mode it is never called: describing a write must
// not open a pool, start a transaction, or even dial the host.
//
// Implementations must be idempotent — the runtime may call Connect on a module
// that is already connected.
type ConnectableModule interface {
	Module

	// Connect opens and validates the connection to the destination.
	Connect(ctx context.Context) error
}

// httpOperationPreview wraps an HTTP request description as an operation
// preview, so the HTTP-based modules keep building previews in one line.
func httpOperationPreview(endpoint, method string, headers map[string]string, bodyPreview string, recordCount int) connector.OperationPreview {
	return connector.OperationPreview{
		Kind:        connector.PreviewKindHTTP,
		RecordCount: recordCount,
		HTTP: &connector.HTTPPreview{
			Endpoint:    endpoint,
			Method:      method,
			Headers:     headers,
			BodyPreview: bodyPreview,
		},
	}
}

// PreviewOptions configures preview generation behavior.
//
// Example usage:
//
//	opts := PreviewOptions{
//	    ShowCredentials: false, // Default: mask credentials for security
//	}
//	previews, err := module.PreviewOperations(records, opts)
type PreviewOptions struct {
	// ShowCredentials when true displays actual credentials instead of masked values.
	// When false (default), authentication headers are masked as [MASKED-TOKEN], [MASKED-API-KEY], etc.
	// WARNING: Only set to true for debugging in secure environments.
	// Never enable this in production or when sharing preview output.
	ShowCredentials bool
}

// PreviewableModule extends Module with preview capability for dry-run mode.
//
// This is an optional extension interface that modules can implement to support
// dry-run mode. Modules that implement PreviewableModule can show what requests
// would be sent without actually sending them.
//
// # Interface Composition
//
// PreviewableModule uses interface composition to extend Module:
//   - Modules must implement both Module and PreviewableModule
//   - The runtime checks for PreviewableModule using a type assertion
//   - If not implemented, the runtime falls back to regular Module behavior
//
// # When to Implement
//
// Implement PreviewableModule if your output module writes anywhere the author
// would want to check before the fact — an HTTP API, a database, a queue. The
// preview type is protocol-neutral (connector.OperationPreview), so a module
// does not have to describe itself as an HTTP request to be previewable.
//
// # Dry-Run Mode
//
// In dry-run mode, the runtime will:
//   - Call PreviewOperations() instead of Send()
//   - Display the preview to the user
//   - Skip actual data transmission
//
// This is useful for:
//   - Validating pipeline configuration
//   - Debugging data transformations
//   - Reviewing what would be sent before production execution
//
// # Stability
//
// This interface is designed to remain stable across versions.
// It is an optional extension - modules are not required to implement it.
type PreviewableModule interface {
	Module

	// PreviewOperations prepares operation previews without performing them.
	//
	// This method should describe the same operations that Send() would perform,
	// without the network call, the connection, or the write.
	//
	// Returns one preview per operation that would be performed:
	//   - Batch mode: returns 1 preview for all records
	//   - Single record mode: returns N previews (one per record)
	//
	// The opts parameter controls preview behavior:
	//   - ShowCredentials: when true, displays actual auth values (use with caution)
	//     When false (default), authentication headers are masked for security.
	//
	// Implementations should respect ShowCredentials to avoid exposing sensitive data.
	PreviewOperations(records []map[string]any, opts PreviewOptions) ([]connector.OperationPreview, error)
}
