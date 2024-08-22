package replacepathregex

import (
	"context"
	"http-benchmark/pkg/config"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"
)

func TestReplacePathRegexMiddleware_ServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		regex          string
		replacement    string
		originalPath   string
		expectedPath   string
		expectedHeader string
	}{
		{
			name:           "Replace /api/v1 with /v1",
			regex:          "^/api(/v1.*)",
			replacement:    "$1",
			originalPath:   "/api/v1/users",
			expectedPath:   "/v1/users",
			expectedHeader: "/api/v1/users",
		},
		{
			name:           "No replacement needed",
			regex:          "^/api(/v2.*)",
			replacement:    "$1",
			originalPath:   "/v1/users",
			expectedPath:   "/v1/users",
			expectedHeader: "/v1/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewMiddleware(tt.regex, tt.replacement)

			ctx := app.NewContext(0)
			ctx.Request.SetRequestURI(tt.originalPath)

			middleware.ServeHTTP(context.Background(), ctx)

			assert.Equal(t, tt.expectedPath, string(ctx.Request.URI().Path()), "Path should be replaced correctly")
			assert.Equal(t, tt.expectedHeader, ctx.Request.Header.Get("X-Replaced-Path"), "Original path should be set in header")

			originalPathFromContext, exists := ctx.Get(config.REQUEST_PATH)
			assert.True(t, exists, "Original path should be set in context")
			assert.Equal(t, tt.originalPath, originalPathFromContext, "Original path in context should match")
		})
	}
}
