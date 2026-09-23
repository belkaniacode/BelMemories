package main

/*
#cgo pkg-config: gtk+-3.0
#include <stdlib.h>
void bm_open_uri(const char *uri, long id);
*/
import "C"

import (
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

var (
	openSeq     atomic.Int64
	openMu      sync.Mutex
	openPending = map[int64]chan error{}
)

//export bmOpenDone
func bmOpenDone(id C.long, msg *C.char) {
	openMu.Lock()
	ch := openPending[int64(id)]
	delete(openPending, int64(id))
	openMu.Unlock()
	if ch == nil {
		return
	}
	if msg != nil {
		ch <- errors.New(C.GoString(msg))
	} else {
		ch <- nil
	}
}

// openWithActivation opens path with the default app via GTK on the UI thread.
// Unlike a bare xdg-open, GTK attaches an XDG activation token, so on Wayland
// the file manager may come to the front — also when the folder is already
// open in an existing window (it is raised instead of silently staying behind).
func openWithActivation(path string) error {
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	id := openSeq.Add(1)
	ch := make(chan error, 1)
	openMu.Lock()
	openPending[id] = ch
	openMu.Unlock()

	cs := C.CString(uri)
	defer C.free(unsafe.Pointer(cs))
	C.bm_open_uri(cs, C.long(id))

	select {
	case err := <-ch:
		return err
	case <-time.After(10 * time.Second):
		openMu.Lock()
		delete(openPending, id)
		openMu.Unlock()
		return errors.New("timeout waiting for the GTK main loop")
	}
}
