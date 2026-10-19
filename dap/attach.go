/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v3.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package dap

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/apiarytech/beedance/vmdebug"
)

// AttachArgs are the attach request's arguments: what the host's Attach
// gets, besides what the editor's configuration adds.
type AttachArgs struct {
	// Program names the program to attach to, as the host knows it.
	Program string `json:"program"`
	// File is the path of the source file the editor shows for it: the
	// breakpoints' source, and what the stack trace opens. Empty: the
	// program's name, with ".st".
	File string `json:"file"`
	// Source, if given, is that file's text: the host may refuse to
	// attach when it is not what the program runs from, so the lines
	// the editor shows are the lines that run.
	Source string `json:"source"`
	// StopOnEntry stops the program before its next scan.
	StopOnEntry bool `json:"stopOnEntry"`
}

// Attacher is what a host with programs of its own gives the adapter
// (Server.Attach): a session attached to the program AttachArgs names
// (vmdebug.Attach). The adapter detaches it when the editor disconnects.
type Attacher func(a AttachArgs) (*vmdebug.Session, error)

var errNoAttach = errors.New("beedance debugs the program it launches; to attach to a running one, debug through its host (beehive)")

// attach answers an attach request through s.Attach.
func (s *Server) attach(a AttachArgs) error {
	if s.Attach == nil {
		return errNoAttach
	}
	if s.sess != nil {
		return errors.New("attach: a session is in progress")
	}
	sess, err := s.Attach(a)
	if err != nil {
		return err
	}
	path := a.File
	if path == "" {
		path = a.Program + ".st"
	}
	if a.Program == "" {
		a.Program = sess.Program()
	}
	s.sess, s.path = sess, filepath.Clean(path)
	s.args = launchArgs{Program: path, ProgramName: a.Program, StopOnEntry: a.StopOnEntry}
	sess.Live()
	if len(s.pending) > 0 {
		s.applyBreakpoints(s.pending)
	}
	s.output("console", fmt.Sprintf("beedance: attached to PROGRAM %s as it runs. A breakpoint stops it, and its outputs, "+
		"until it goes on; each step is one of its own cycles. In this console: force NAME VALUE, unforce NAME, forces, scan, live, or an expression.",
		sess.Program()))
	return nil
}

// detach lets an attached program go, if the session is one.
func (s *Server) detach() {
	if s.sess != nil && s.sess.Attached() {
		s.sess.Detach()
	}
}
