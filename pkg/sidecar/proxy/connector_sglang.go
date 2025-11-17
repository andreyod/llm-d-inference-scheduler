/*
Copyright 2025 The llm-d Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"time"
)

func (s *Server) runSGLangProtocol(w http.ResponseWriter, r *http.Request, prefillPodHostPort string) {
	// s.logger.V(4).Info("running SGLang protocol", "url", prefillPodHostPort)
	s.logger.Info("running SGLang protocol", "url", prefillPodHostPort)

	// Make Request
	requestData, err := s.parseSGLangRequest(r)

	if err != nil {
		if err := errorJSONInvalid(err, w); err != nil {
			s.logger.Error(err, "failed to send error response to client")
		}
		return
	}

	// Validate prefill host
	if prefillPodHostPort == "" {
		err := fmt.Errorf("prefill host required for SGLang P/D disaggregation")
		if err := errorJSONInvalid(err, w); err != nil {
			s.logger.Error(err, "failed to send error response to client")
		}
		return
	}

	roomID := s.generateSGLangRoomID()

	// Inject bootstrap info for both prefill and decode
	bootstrapInfo := s.addSGLangBootstrapInfo(requestData, prefillPodHostPort, roomID)

	body, err := json.Marshal(bootstrapInfo)
	if err != nil {
		if err := errorJSONInvalid(err, w); err != nil {
			s.logger.Error(err, "failed to send error response to client")
		}
		return
	}

	newReq := r.Clone(r.Context())
	newReq.Body = io.NopCloser(strings.NewReader(string(body)))
	newReq.ContentLength = int64(len(body))
	newReq.Header.Set("Content-Type", "application/json")

	// Send concurrent prefill and decode requests
	s.sendSGLangConcurrentRequests(w, newReq, prefillPodHostPort)

	// Send prefill and decode requests
	//s.serveSGLangPDRequests(w, newReq, prefillPodHostPort)

	// TEMP ignore bootstrap info
	// s.sendSGLangConcurrentRequests(w, r, prefillPodHostPort)
}

// func (s *Server) serveSGLangPDRequests(w http.ResponseWriter, r *http.Request, prefillHost string) {
// 	prefillHandler, err := s.prefillerProxyHandler(prefillHost)
// 	if err != nil {
// 		s.logger.Error(err, "failed to get prefiller proxy handler", "prefill_host", prefillHost)
// 		return
// 	}
// 	pw := &bufferedResponseWriter{}

// 	dump, err := httputil.DumpRequest(r, true) // 'true' to include the body
// 	if err != nil {
// 		s.logger.Info("--------------------- Dump request failed", "error", err)
// 	}
// 	s.logger.V(5).Info("---- request before copy", "--------------- r:", dump)

// 	// Serve the prefill request
// 	s.logger.V(5).Info("sending request to prefiller", "destination", prefillHost)
// 	go func() {
// 		prefillHandler.ServeHTTP(pw, r)
// 		if pw.statusCode < http.StatusOK || pw.statusCode >= http.StatusMultipleChoices {
// 			s.logger.Error(fmt.Errorf("prefill request failed with status %d", pw.statusCode), "prefill request error")
// 			return
// 		}
// 		s.logger.V(5).Info("received prefiller response", "status code", pw.statusCode, "body", pw.buffer.String())
// 	}()

// 	// Serve the decode request
// 	s.logger.V(5).Info("serving decode request", "url", s.decoderURL.String())
// 	s.decoderProxy.ServeHTTP(w, r)
// }

func (s *Server) sendSGLangConcurrentRequests(w http.ResponseWriter, r *http.Request, prefillHost string) {
	Req := r.Clone(r.Context())
	Req.Body = r.Body
	Req.ContentLength = r.ContentLength

	// Send prefill request asynchronously
	go func() {
		prefillHandler, err := s.prefillerProxyHandler(prefillHost)
		if err != nil {
			s.logger.Error(err, "failed to get prefiller proxy handler", "prefill_host", prefillHost)
			return
		}
		// pw := &bufferedResponseWriter{}
		// Debug --------------------------------------------------------------
		tw := httptest.NewRecorder()
		dump, err := httputil.DumpRequest(r, true) // 'true' to include the body
		if err != nil {
			s.logger.Info("--------------------- Dump request failed", "error", err)
		}
		s.logger.V(5).Info("---- request before copy", "--------------- r:", dump)
		s.logger.Info("----------------------------------------------------------")

		dump, err = httputil.DumpRequest(Req, true) // 'true' to include the body
		if err != nil {
			s.logger.Info("--------------------- Dump request failed", "error", err)
		}
		s.logger.V(5).Info("---- sending prefill request", "--------------- Req:", dump)
		// ---------------------------------------------------------------------
		// prefillHandler.ServeHTTP(pw, Req)
		// s.logger.V(5).Info("prefill request completed", "status", pw.statusCode)

		//prefillHandler.ServeHTTP(tw, Req)
		prefillHandler.ServeHTTP(tw, r)
		s.logger.V(5).Info("---- prefill request completed", "status", tw.Code, "body: ", tw.Body.String())
	}()
	// Debug --------------------------------------------------------------
	s.logger.V(5).Info("sending decode request", "---------------:")
	// ----------------------------------------------------------------------
	// Send decode request synchronously
	//s.decoderProxy.ServeHTTP(w, Req)
	s.decoderProxy.ServeHTTP(w, r)
	// Debug --------------------------------------------------------------
	s.logger.V(5).Info("got decode response", "---------------:", w)
}

func (s *Server) addSGLangBootstrapInfo(requestData map[string]interface{}, prefillHostPort string, roomID int64) map[string]interface{} {
	modifiedRequest := make(map[string]interface{})
	for k, v := range requestData {
		modifiedRequest[k] = v
	}

	// Generate bootstrap host from prefill host
	bootstrapHost, bootstrapPort := s.getBootstrapHost(prefillHostPort)

	// Add bootstrap information
	modifiedRequest[requestFieldBootstrapHost] = bootstrapHost
	modifiedRequest[requestFieldBootstrapPort] = bootstrapPort
	modifiedRequest[requestFieldBootstrapRoom] = roomID

	// s.logger.V(5).Info("bootstrap info added",
	s.logger.Info("bootstrap info added",
		"bootstrap_host", bootstrapHost,
		"bootstrap_port", bootstrapPort,
		"bootstrap_room", roomID)

	return modifiedRequest
}

func (s *Server) parseSGLangRequest(r *http.Request) (map[string]interface{}, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		return nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	return requestData, nil
}

func (s *Server) generateSGLangRoomID() int64 {
	return time.Now().UnixNano() + int64(rand.Intn(1000))
}

func (s *Server) getBootstrapHost(prefillHostPort string) (string, int) {
	// Extract hostname from prefill host
	parts := strings.Split(prefillHostPort, ":")
	hostname := parts[0]
	// Get bootstrap port from environment variable
	bootstrapPort := 8998 // Default SGLang bootstrap port
	if portStr := os.Getenv("SGLANG_BOOTSTRAP_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			bootstrapPort = port
		}
	}
	return hostname, bootstrapPort
}
