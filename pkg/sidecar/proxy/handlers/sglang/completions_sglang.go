package sglang

import (
	"net/http"

	"github.com/go-logr/logr"
	lru "github.com/hashicorp/golang-lru/v2"
	handlers "github.com/llm-d/llm-d-inference-scheduler/pkg/sidecar/proxy/handlers"
)

type SGLangHandlers struct {
	logger             logr.Logger
	allowlistValidator *AllowlistValidator
	config             Config
	decoderProxy       http.Handler                     // decoder proxy handler
	prefillerProxies   *lru.Cache[string, http.Handler] // cached prefiller proxy handlers
}

func NewSGLangHandlers(config Config, decoderProxy http.Handler) *SGLangHandlers {
	return &SGLangHandlers{
		config:       config,
		decoderProxy: decoderProxy,
	}
}

func (n *SGLangHandlers) ChatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	// Flow: 1. get prefill host 2. Send to Prefill+Decoder concurently

	prefillHostPort := handlers.GetPrefillHostPortFromHeader(r.Header.Values("X-Prefill-Pod"))

	n.preparePrefillRequest(r)

	n.prepareDecodeRequest(r)

	go func() {
		handlers.RunPrefill(prefillHostPort, w, r)
	}()

	handlers.RunDecode(n.decoderProxy, w, r)
}
