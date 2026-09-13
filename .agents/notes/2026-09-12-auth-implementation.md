Auth shipped on feat/orchestrator-core (issue #36, commits b373e36..bb677fa):

- internal/auth: bearer token, env MCPSIM_AUTH_TOKEN > server.auth.token yaml > first-boot generated at ~/.config/mcp-sim/token (0600) + client-snippet.json. SDK go-sdk@v1.6.1 auth.RequireBearerToken middleware (NOT homegrown): spec-shaped WWW-Authenticate + TokenInfo.UserID session binding.
- SDK gotchas: WWW-Authenticate challenge only emitted when RequireBearerTokenOptions.ResourceMetadataURL set; verifier MUST set TokenInfo.Expiration non-zero (used now+100y for static token); SDK sets the header before wrapped handler so it can appear on 200s.
- Gating: --insecure-no-auth refused on non-loopback (empty host in :9090 and 0.0.0.0 count as non-loopback) without --insecure-no-auth-ack or MCPSIM_TRUSTED_NETWORK=true.
- Coverage closed: hermetic httptest integration (bootstrap_http_test.go) runs real mux+middleware+SDK streamable handler; tool results nest under result.structuredContent; responses may be SSE (parse data: lines) or JSON depending on Accept.
- Tailnet POC on quinos-mac-mini worked end to end (401 -> authorized 11 tools -> SIGTERM clean). simctl fails there: only Xcode CLT, exit 72, no full Xcode. Install Xcode for iOS on the mini, or adb/emulator for Android.
- Project 20 field gotchas: Quarter iteration ids Q2=b64ce631 Q3=a0844f8e (mixed up once); item-list jq keys are 'start date'/'target date' (with spaces); Iteration field (PVTIF...T8) has zero iterations configured, only Quarter (PVTIF...UA) is usable.
