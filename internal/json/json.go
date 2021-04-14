// Copyright 2017 Bo-Yi Wu.  All rights reserved.
// This code is copied and pasted from gin.

//go:build !jsoniter
// +build !jsoniter

package json

import "encoding/json"

var (
	Marshal       = json.Marshal
	Unmarshal     = json.Unmarshal
	MarshalIndent = json.MarshalIndent
	NewDecoder    = json.NewDecoder
	NewEncoder    = json.NewEncoder
)
