package chrome

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Process is an active running chrome binary.
type Process struct {
	cmd *exec.Cmd
	// buf is the combined process stdout and stderr.
	buf *bytes.Buffer
	// addr that the underlying chrome binary is listening on.
	addr *net.TCPAddr
	// cancel cancels the process context.
	cancel context.CancelFunc
	// err is set once process exits and done closes, always of type *ProcessError.
	err error
	// done is closed once the process exits and err is set.
	done chan struct{}
	// tmpDir used for temporary data storage.
	tmpDir string
}

// Addr in host:port format that the chrome debug server is listening on.
func (p *Process) Addr() string {
	return p.addr.String()
}

// Done is closed once the process exits. Error is always set when Done is closed.
func (p *Process) Done() <-chan struct{} {
	return p.done
}

// Err is set when Done closes.
func (p *Process) Err() error {
	return p.err
}

// Stops sends the signal to stop the process. If successful, Stop returns nil.
func (p *Process) Stop() error {
	p.cancel()
	<-p.done
	if p.err == ErrProcessExited {
		return nil
	}
	return p.err
}

// ErrProcessExited signals that the process exited gracefully with exit code 0.
var ErrProcessExited = errors.New("process exited")

// ProcessError contains additional info for the process that failed.
type ProcessError struct {
	// Args used to call the process; first argument is the binary.
	Args []string
	// Out is the combined stdout and stderr.
	Out []byte
	// Err is the process exit error.
	Err error
}

func (e *ProcessError) Error() string {
	return fmt.Sprintf("command %s failed: %s\nOUTPUT:\n%s", strings.Join(e.Args, " "), e.Err, string(e.Out))
}

// Start runs a google-chrome binary in a newly set up temporary directory.
// The process is debug server is listening on Addr. Call Close to exit the
// process and clean up the temporary directory.
func Start() (*Process, error) {
	dir, err := ioutil.TempDir("", "chrome_headless_")
	_ = dir
	if err != nil {
		return nil, err
	}
	binary, err := exec.LookPath("google-chrome")
	if err != nil {
		return nil, err
	}
	addr, err := reserveListenAddr("tcp", "127.0.0.1")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	buf := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(ctx, binary, "--headless", "--disable-gpu", "--remote-debugging-address="+addr.IP.String(), "--remote-debugging-port="+strconv.Itoa(addr.Port), "https://example.com")
	readyMessage := `DevTools listening on`
	waiter := &writeSignaler{buf: buf, content: []byte(readyMessage), signalCh: make(chan struct{})}
	cmd.Stdout = waiter
	cmd.Stderr = waiter
	proc := &Process{
		cmd:    cmd,
		buf:    buf,
		addr:   addr,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go runProcess(ctx, proc)
	select {
	case <-time.After(15 * time.Second):
		err = fmt.Errorf("timed out while waiting for \"%s\" in output", readyMessage)
		proc.cancel()
		<-proc.done
		return nil, &ProcessError{Args: cmd.Args, Out: buf.Bytes(), Err: err}
	case <-proc.done:
		err = proc.err
		if err == nil {
			err = errors.New("process exited early")
		}
		proc.cancel()
		<-proc.done
		return nil, &ProcessError{Args: cmd.Args, Out: buf.Bytes(), Err: err}
	case <-waiter.signalCh:
		return proc, nil
	}
}

// CleanupError signifies that the temporary directory could not be cleaned up.
type CleanupError struct {
	// Dir that failed to be cleaned up.
	Dir string
	// Err is the cleanup error.
	Err error
	// ProcErr is the process exit error.
	ProcErr error
}

func (e *CleanupError) Error() string {
	return fmt.Sprintf("erorr cleaning up directory \"%s\": %s; process error: %s", e.Dir, e.Err, e.ProcErr)
}

// runProcess runs the process, captures its exit error, cleans up the temporary
// directory, and closes its done channel once the exit error is set.
func runProcess(ctx context.Context, proc *Process) {
	proc.err = proc.cmd.Run()
	select {
	case <-ctx.Done():
		proc.err = ErrProcessExited
	default:
	}
	if len(proc.tmpDir) != 0 {
		if err := os.RemoveAll(proc.tmpDir); err != nil {
			proc.err = &CleanupError{Dir: proc.tmpDir, ProcErr: proc.err, Err: err}
		}
	}
	proc.cancel()
	close(proc.done)
}

// writeSignaler closes signalCh once its internal buffer contains specified content.
// It is not safe for concurrent use.
type writeSignaler struct {
	buf      *bytes.Buffer
	content  []byte
	signalCh chan struct{}
}

func (w *writeSignaler) Write(p []byte) (int, error) {
	n, err := w.buf.Write(p)
	select {
	case <-w.signalCh:
	default:
		if bytes.Contains(w.buf.Bytes(), w.content) {
			close(w.signalCh)
		}
	}
	return n, err
}

// reserveListenAddr opens a random OS TCP port and immediately closes it,
// hopefully making it available for reservation for some time.
func reserveListenAddr(network, ip string) (*net.TCPAddr, error) {
	ln, err := net.Listen(network, ip+":0")
	if err != nil {
		return nil, err
	}
	lnAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("expected *net.TCPAddr, got %T", ln.Addr())
	}
	if err = ln.Close(); err != nil {
		return nil, err
	}
	return lnAddr, nil
}
