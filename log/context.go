package log

import (
	"context"
	"fmt"
)

type CtxKey string

const (
	CtxKeyLabels CtxKey = "WBCLIENT_LOG_LABELS"
)

// Labels attached to labels attribute when the log format is json
type Labels map[string]string

// Context returns new context given parent and set of labels of the form (label-1, value-1....label-n, value-n)
func Context(parent context.Context, labels ...string) context.Context {
	mustValidateLabels(labels...)

	ctxLabels := LabelsFromCtx(parent)
	if len(labels) != 0 {
		for i := 0; i < len(labels); i += 2 {
			ctxLabels[labels[i]] = labels[i+1]
		}
	}

	return context.WithValue(parent, CtxKeyLabels, ctxLabels)
}

// LabelsFromContext returns the Labels from ctx.
func LabelsFromCtx(ctx context.Context) Labels {
	if ctx == nil {
		return make(Labels)
	}

	labels, ok := ctx.Value(CtxKeyLabels).(Labels)
	if !ok {
		return make(Labels)
	}

	resp := make(Labels)
	for key, value := range labels {
		resp[key] = value
	}

	return resp
}

func LabelFromCtx(ctx context.Context, label string) string {
	labels := LabelsFromCtx(ctx)
	return labels[label]
}

func labelFieldsFromCtx(ctx context.Context) (labels []string) {
	for key, value := range LabelsFromCtx(ctx) {
		labels = append(labels, key, value)
	}

	return labels
}

func mustValidateLabels(labels ...string) {
	if len(labels) != 0 && len(labels)%2 != 0 {
		panic(fmt.Sprintf("log labels[%v] should be of even count", labels))
	}
}
