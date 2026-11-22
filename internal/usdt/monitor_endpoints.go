package usdt

import "context"

// scanEndpoints keeps the last successful endpoint first. A failed preferred
// endpoint otherwise adds its timeout to every subsequent confirmation scan.
// All attempts still use the shared bounded executor and the durable cursor.
func scanEndpoints(ctx context.Context, readers []*scanner, cursor Cursor, addresses []string) (Batch, *scanner, error) {
	lastErr := ErrUnavailable
	for index, reader := range readers {
		if err := ctx.Err(); err != nil {
			return Batch{}, nil, err
		}
		batch, err := reader.scan(ctx, cursor, addresses)
		if err != nil {
			reader.closeSubscription()
			lastErr = err
			continue
		}
		copy(readers[1:index+1], readers[:index])
		readers[0] = reader
		closeInactiveSubscriptions(readers, reader)
		return batch, reader, nil
	}
	return Batch{}, nil, lastErr
}
