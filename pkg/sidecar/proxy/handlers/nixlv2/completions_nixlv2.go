package nixlv2

import (
	"net/http"

	"github.com/go-logr/logr"
	lru "github.com/hashicorp/golang-lru/v2"
	handlers "github.com/llm-d/llm-d-inference-scheduler/pkg/sidecar/proxy/handlers"
)

type Nixlv2Handlers struct {
	logger             logr.Logger
	allowlistValidator *AllowlistValidator
	config             Config
	decoderProxy       http.Handler                     // decoder proxy handler
	prefillerProxies   *lru.Cache[string, http.Handler] // cached prefiller proxy handlers
}

func NewNixlv2Handlers(config Config, decoderProxy http.Handler) *Nixlv2Handlers {
	return &Nixlv2Handlers{
		config:       config,
		decoderProxy: decoderProxy,
	}
}

func (n *Nixlv2Handlers) ChatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	// Flow: 1. get prefill host 2. Send to Prefiller  3. Proxy to Decoder

	prefillHostPort := handlers.GetPrefillHostPortFromHeader(r.Header.Values("X-Prefill-Pod"))

	n.preparePrefillRequest(r)

	handlers.RunPrefill(prefillHostPort, w, r)

	n.prepareDecodeRequest(r)

	handlers.RunDecode(n.decoderProxy, w, r)
}
