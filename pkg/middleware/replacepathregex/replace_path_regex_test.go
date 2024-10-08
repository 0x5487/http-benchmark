package replacepathregex

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"
)

func TestReplacePathRegexMiddleware_ServeHTTP(t *testing.T) {
	tests := []struct {
		name             string
		regex            string
		replacement      string
		originalPath     string
		originalFullPath string
		expectedPath     string
		expectedFullPath string
		expectedHeader   string
	}{
		{
			name:             "Replace path",
			regex:            "^/api/v1/(.*)$",
			replacement:      "/hoo/$1",
			originalPath:     "/api/v1/users",
			originalFullPath: "/api/v1/users?name=john",
			expectedPath:     "/hoo/users",
			expectedFullPath: "/hoo/users?name=john",
			expectedHeader:   "/api/v1/users",
		},
		{
			name:             "No replacement needed",
			regex:            "^/api(/v2.*)",
			replacement:      "$1",
			originalPath:     "/v1/users",
			originalFullPath: "/v1/users",
			expectedPath:     "/v1/users",
			expectedFullPath: "/v1/users",
			expectedHeader:   "/v1/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewMiddleware(tt.regex, tt.replacement)

			ctx := app.NewContext(0)
			ctx.Request.SetRequestURI(tt.originalFullPath)

			middleware.ServeHTTP(context.Background(), ctx)

			assert.Equal(t, tt.expectedPath, string(ctx.Request.URI().Path()), "Path should be replaced correctly")
			assert.Equal(t, tt.expectedFullPath, string(ctx.Request.URI().RequestURI()), "Full Path should be replaced correctly")
		})
	}
}
