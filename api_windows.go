package termbox

import "syscall"

// public API

// Initializes termbox library. This function should be called before any other functions.
// After successful initialization, the library must be finalized using 'Close' function.
//
// Example usage:
//      err := termbox.Init()
//      if err != nil {
//              panic(err)
//      }
//      defer termbox.Close()
func Init() error {
	var err error

	in, err = syscall.GetStdHandle(syscall.STD_INPUT_HANDLE)
	if err != nil {
		return err
	}
	out, err = syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return err
	}

	err = get_console_mode(in, &orig_mode)
	if err != nil {
		return err
	}

	err = set_console_mode(in, enable_window_input)
	if err != nil {
		return err
	}

	go input_event_producer()

	return nil
}

// Finalizes termbox library, should be called after successful initialization
// when termbox's functionality isn't required anymore.
func Close() {
	// we ignore errors here, because we can't really do anything about them
	set_console_mode(in, orig_mode)
	set_console_active_screen_buffer(orig_screen)
}

// Synchronizes the internal back buffer with the terminal.
func Flush() error {
	update_size_maybe()
	prepare_diff_messages()
	for _, msg := range diffbuf {
		write_console_output_attribute(out, msg.attrs, msg.pos)
		write_console_output_character(out, msg.chars, msg.pos)
	}
	if !is_cursor_hidden(cursor_x, cursor_y) {
		move_cursor(cursor_x, cursor_y)
	}
	return nil
}

// Wait for an event and return it. This is a blocking function call.
func PollEvent() Event {
	return <-input_comm
}

// Returns the size of the internal back buffer (which is the same as
// terminal's window size in characters).
func Size() (int, int) {
	return termw, termh
}

// Sets termbox input mode. Termbox has two input modes:
//
// 1. Esc input mode. When ESC sequence is in the buffer and it doesn't match
// any known sequence. ESC means KeyEsc.
//
// 2. Alt input mode. When ESC sequence is in the buffer and it doesn't match
// any known sequence. ESC enables ModAlt modifier for the next keyboard event.
//
// If 'mode' is InputCurrent, returns the current input mode. See also Input*
// constants.
func SetInputMode(mode InputMode) InputMode {
	if mode != InputCurrent {
		input_mode = mode
	}
	return input_mode
}
