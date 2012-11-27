// +build !windows

package termbox

import "unicode/utf8"
import "syscall"
import "unsafe"
import "strings"
import "os"

// private API

const (
	t_enter_ca = iota
	t_exit_ca
	t_show_cursor
	t_hide_cursor
	t_clear_screen
	t_sgr0
	t_underline
	t_bold
	t_blink
	t_reverse
	t_enter_keypad
	t_exit_keypad
	t_max_funcs
)

type input_event struct {
	data []byte
	err  error
}

var (
	// term specific sequences
	keys  []string
	funcs []string

	// termbox inner state
	orig_tios  syscall_Termios
	termw      int
	termh      int
	input_mode = InputEsc
	out        *os.File
	in         int
	inbuf      = make([]byte, 0, 64)
	sigwinch   = make(chan os.Signal, 1)
	sigio      = make(chan os.Signal, 1)
	quit       = make(chan int)
	input_comm = make(chan input_event)
	intbuf     = make([]byte, 0, 16)
)

type winsize struct {
	rows    uint16
	cols    uint16
	xpixels uint16
	ypixels uint16
}

func get_term_size(fd uintptr) (int, int) {
	var sz winsize
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL,
		fd, uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&sz)))
	return int(sz.cols), int(sz.rows)
}

func tcsetattr(fd uintptr, termios *syscall_Termios) error {
	r, _, e := syscall.Syscall(syscall.SYS_IOCTL,
		fd, uintptr(syscall_TCSETS), uintptr(unsafe.Pointer(termios)))
	if r != 0 {
		return os.NewSyscallError("SYS_IOCTL", e)
	}
	return nil
}

func tcgetattr(fd uintptr, termios *syscall_Termios) error {
	r, _, e := syscall.Syscall(syscall.SYS_IOCTL,
		fd, uintptr(syscall_TCGETS), uintptr(unsafe.Pointer(termios)))
	if r != 0 {
		return os.NewSyscallError("SYS_IOCTL", e)
	}
	return nil
}

func parse_escape_sequence(event *Event, buf []byte) int {
	bufstr := string(buf)
	for i, key := range keys {
		if strings.HasPrefix(bufstr, key) {
			event.Ch = 0
			event.Key = Key(0xFFFF - i)
			return len(key)
		}
	}
	return 0
}

func extract_event(event *Event) bool {
	if len(inbuf) == 0 {
		return false
	}

	if inbuf[0] == '\033' {
		// possible escape sequence
		n := parse_escape_sequence(event, inbuf)
		if n != 0 {
			copy(inbuf, inbuf[n:])
			inbuf = inbuf[:len(inbuf)-n]
			return true
		}

		// it's not escape sequence, then it's Alt or Esc, check input_mode
		switch input_mode {
		case InputEsc:
			// if we're in escape mode, fill Esc event, pop buffer, return success
			event.Ch = 0
			event.Key = KeyEsc
			event.Mod = 0
			copy(inbuf, inbuf[1:])
			inbuf = inbuf[:len(inbuf)-1]
			return true
		case InputAlt:
			// if we're in alt mode, set Alt modifier to event and redo parsing
			event.Mod = ModAlt
			copy(inbuf, inbuf[1:])
			inbuf = inbuf[:len(inbuf)-1]
			return extract_event(event)
		default:
			panic("unreachable")
		}
	}

	// if we're here, this is not an escape sequence and not an alt sequence
	// so, it's a FUNCTIONAL KEY or a UNICODE character

	// first of all check if it's a functional key
	if Key(inbuf[0]) <= KeySpace || Key(inbuf[0]) == KeyBackspace2 {
		// fill event, pop buffer, return success
		event.Ch = 0
		event.Key = Key(inbuf[0])
		copy(inbuf, inbuf[1:])
		inbuf = inbuf[:len(inbuf)-1]
		return true
	}

	// the only possible option is utf8 rune
	if r, n := utf8.DecodeRune(inbuf); r != utf8.RuneError {
		event.Ch = r
		event.Key = 0
		copy(inbuf, inbuf[n:])
		inbuf = inbuf[:len(inbuf)-n]
		return true
	}

	return false
}

func fcntl(fd int, cmd int, arg int) (val int, err error) {
	r, _, e := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(cmd),
		uintptr(arg))
	val = int(r)
	if e != 0 {
		err = e
	}
	return
}
