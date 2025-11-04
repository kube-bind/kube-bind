/*
Copyright 2022 The Kube Bind Authors.

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

package session

import (
	"github.com/vmihailenco/msgpack/v4"
)

// State is the data stored on the clientside as a browser cookie.
// To be independent of the OIDC token sizes, it only stores the absolute minimum
// information required for this particular backend implementation. Especially
// when lots of groups and claims are involved, the tokens can grow to a size
// larger than allowed by browsers for cookies, and even compression would only
// be a small adhesive strip, not a solution.
type State struct {
	Token       TokenInfo `msgpack:"tok,omitempty"`
	SessionID   string    `msgpack:"sid,omitempty"`
	ClusterID   string    `msgpack:"cid,omitempty"`
	RedirectURL string    `msgpack:"red,omitempty"`
}

type TokenInfo struct {
	Subject string `msgpack:"sub,omitempty"`
	Issuer  string `msgpack:"iss,omitempty"`
}

func (s *State) Encode() ([]byte, error) {
	return msgpack.Marshal(s)
}
