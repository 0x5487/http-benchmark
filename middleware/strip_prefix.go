package middleware

import (
	"bytes"
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

type StripPrefixMiddleware struct {
	prefixes [][]byte
}

func NewStripPrefixMiddleware(prefixs []string) *StripPrefixMiddleware {
	m := &StripPrefixMiddleware{
		prefixes: make([][]byte, 0),
	}
	for _, prefix := range prefixs {
		m.prefixes = append(m.prefixes, []byte(prefix))
	}

	return m
}

func (m *StripPrefixMiddleware) ServeHTTP(c context.Context, ctx *app.RequestContext) {

	for _, prefix := range m.prefixes {
		if bytes.HasPrefix(ctx.Request.Path(), prefix) {
			newPath := bytes.TrimPrefix(ctx.Request.Path(), prefix)
			ctx.Request.URI().SetPathBytes(newPath)
			break
		}
	}

	ctx.Next(c)
}
