package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
)

func getenvDefault(k, def string) string {
	v := os.Getenv(k)
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func prettyJSON(b []byte) string {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(b)
	}
	return string(out)
}

func parseKeyValues(args []string) map[string]string {
	out := map[string]string{}
	for _, a := range args {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if k != "" {
			out[k] = v
		}
	}
	return out
}

//
// context helpers for shared API client
//

type ctxKey string

const apiKey ctxKey = "api"

func WithAPI(ctx context.Context, api *API) context.Context {
	return context.WithValue(ctx, apiKey, api)
}

func GetAPI(ctx context.Context) *API {
	v := ctx.Value(apiKey)
	if v == nil {
		return nil
	}
	return v.(*API)
}
