package domain

import "context"

// ReceiptCostReader exposes settled accounting to trusted domain consumers.
// It must reject a partial receipt set instead of reporting a smaller cost.
type ReceiptCostReader interface {
	SumReceiptCostMicros(context.Context, string, []string) (int64, error)
}

// OperationUsageReader recovers accounting by a persisted command/node key.
// It performs no model dispatch and cannot turn an unknown result into success.
type OperationUsageReader interface {
	FindOperationUsage(context.Context, string, string) (*Usage, error)
}
