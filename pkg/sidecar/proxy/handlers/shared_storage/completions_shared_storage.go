package sharedstorage

import (
	"net/http"

	"github.com/go-logr/logr"
	lru "github.com/hashicorp/golang-lru/v2"
	handlers "github.com/llm-d/llm-d-inference-scheduler/pkg/sidecar/proxy/handlers"
)

type SharedStorageHandlers struct {
	logger             logr.Logger
	allowlistValidator *AllowlistValidator
	config             Config
	decoderProxy       http.Handler                     // decoder proxy handler
	prefillerProxies   *lru.Cache[string, http.Handler] // cached prefiller proxy handlers
}

func NewSharedStorageHandlers(config Config, decoderProxy http.Handler) *SharedStorageHandlers {
	return &SharedStorageHandlers{
		config:       config,
		decoderProxy: decoderProxy,
	}
}

func (n *SharedStorageHandlers) ChatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	// Flow: 1. get prefill host 2. Send to Decoder  3. Proxy to Prefiller

	prefillHostPort := handlers.GetPrefillHostPortFromHeader(r.Header.Values("X-Prefill-Pod"))

	n.prepareDecodeRequest(r)

	handlers.RunDecode(n.decoderProxy, w, r)

	n.preparePrefillRequest(r)

	handlers.RunPrefill(prefillHostPort, w, r)

}
