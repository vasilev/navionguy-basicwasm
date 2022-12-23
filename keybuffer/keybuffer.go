package keybuffer

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/navionguy/basicwasm/ast"
)

const (
	f1Key  = "1b4f50" // the escape sequences received for all 12 function keys
	f2Key  = "1b4f51"
	f3Key  = "1b4f52"
	f4Key  = "1b4f53"
	f5Key  = "1b5b31357e"
	f6Key  = "1b5b31377e"
	f7Key  = "1b5b31387e"
	f8Key  = "1b5b31397e"
	f9Key  = "1b5b32307e"
	f10Key = "1b5b32317e"
	f11Key = "1b5b41" // cursor up
	f12Key = "1b5b44" // cursor left
	f13Key = "1b5b43" // cursor right
	f14Key = "1b5b42" // cursor down
)

type KeyBuffer struct {
	KeySettings *ast.KeySettings
	keycodes    chan ([]byte)
	inp         []byte
	ind         int
	sig_break   bool
	spcKeys     map[string]string
}

var kbuff KeyBuffer

func GetKeyBuffer() *KeyBuffer {
	fkeys := []string{f1Key, f2Key, f3Key, f4Key, f5Key, f6Key, f7Key, f8Key, f9Key, f10Key, f11Key, f12Key, f13Key, f14Key}

	if kbuff.spcKeys == nil {
		kbuff.spcKeys = make(map[string]string)
	}

	for i, key := range fkeys {
		ind := fmt.Sprintf("F%d", i+1)
		kbuff.spcKeys[key] = ind
	}
	return &kbuff
}

// SaveKeyStroke saves all the bytes generated by a keystroke
func (buff *KeyBuffer) SaveKeyStroke(key []byte) {
	// check if my channel has been created
	if buff.keycodes == nil {
		// create a buffered channel
		buff.keycodes = make(chan []byte, 20)
	}

	// check for an escape sequence, like a function key
	if (len(key) > 1) && (key[0] == 0x1b) {
		// go see if maps to something have a macro set for
		key = buff.checkForSpecialKeys(key)
	}

	// check for an empty array
	// special key checks can return an empty array
	if len(key) == 0 {
		return
	}

	// write array to the channel
	buff.keycodes <- key
	// ctrl-c means user wants to stop execution
	buff.checkForCtrlC(key)
}

// checkForCtrlC - looks to flag the user entered a ctrl-c
func (buff *KeyBuffer) checkForCtrlC(inp []byte) {
	for _, k := range inp {
		if k == 0x03 {
			buff.sig_break = true
		}
	}
}

// check for special keys
func (buff *KeyBuffer) checkForSpecialKeys(inp []byte) []byte {
	if buff.KeySettings == nil {
		// no macros have been set
		return []byte("")
	}

	// convert the bytes to a string for checking
	a := kbuff.spcKeys[hex.EncodeToString(inp)]
	mac := kbuff.KeySettings.Keys[a]

	// map the key label to the string to send and return it
	return []byte(mac)
}

// has a Ctrl-C been entered
func (buff *KeyBuffer) BreakSeen() bool {
	time.Sleep(15 * time.Millisecond)
	return buff.sig_break
}

// reset the break flag
func (buff *KeyBuffer) ClearBreak() {
	buff.sig_break = false
}

// ReadByte returns the next byte, caller has to decide if he needs more
func (buff *KeyBuffer) ReadByte() (byte, error) {
	// if I'm working a byte array keep going
	if (buff.inp != nil) && (buff.ind < len(buff.inp)) {
		t := buff.ind
		buff.ind++
		return buff.inp[t], nil
	}

	// if the channel doesn't exist, nothing to report
	if buff.keycodes == nil {
		return ' ', errors.New("no data")
	}

	// try to read key scan codes and repor it
	select {
	case buff.inp = <-buff.keycodes:
		buff.ind = 1
		return buff.inp[0], nil
	default:
		// nothing to report
		return ' ', errors.New("no data")
	}
}
