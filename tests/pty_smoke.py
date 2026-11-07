#!/usr/bin/env python3
"""Offline real-PTY smoke test. Takes a built pi-task binary, never uses real state."""
import errno
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time

BINARY = str(Path(sys.argv[1] if len(sys.argv) > 1 else './bin/pi-task').resolve())


def main():
    with tempfile.TemporaryDirectory(prefix='pi-task-pty-') as root:
        env = dict(os.environ, PI_TASK_HOME=root, TERM='xterm-256color')
        env.pop('NO_COLOR', None)

        def command(*args):
            return subprocess.run([BINARY, *args], env=env, check=True, text=True, capture_output=True).stdout

        for n in range(6):
            command('start', '--id', f'work-{n}', '--title', f'测试工作-{n}', '--task', '检查实现', '--task', '运行验证')
        listing = json.loads(command('status', '--json'))
        assert len(listing['runs']) == 5
        assert listing['runs'][0]['id'] == 'work-5'
        assert 'work-0' not in [run['id'] for run in listing['runs']]

        pid, master = pty.fork()
        if pid == 0:
            os.execve(BINARY, [BINARY], env)
        fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack('HHHH', 24, 80, 0, 0))
        output = bytearray()
        reaped = False

        def pump_until(needle, timeout=5):
            if needle in output:
                return
            deadline = time.monotonic() + timeout
            while time.monotonic() < deadline:
                if select.select([master], [], [], .05)[0]:
                    try:
                        data = os.read(master, 262144)
                    except OSError as e:
                        if e.errno == errno.EIO:
                            break
                        raise
                    if not data:
                        break
                    output.extend(data)
                    if needle in output:
                        return
            raise AssertionError(f'missing {needle!r}: {output.decode(errors="replace")}')

        try:
            pump_until('最近 5 次工作'.encode())
            before = termios.tcgetattr(master)
            # W/S selection, split arrow key input, Enter into selected work.
            os.write(master, b's')
            time.sleep(.12)
            os.write(master, b'w')
            time.sleep(.12)
            os.write(master, b'\x1b[')
            time.sleep(.01)
            os.write(master, b'B')
            time.sleep(.12)
            os.write(master, b'\r')
            pump_until(b'work-4')
            pump_until('检查实现'.encode())
            command('update', '--id', 'work-4', '--task', '1', '--state', 'done', '--next', '2', '--note', 'PTY实时更新已验证')
            pump_until('PTY实时更新已验证'.encode())
            # Narrow and tall resizing while watching must not panic or lose state.
            fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack('HHHH', 12, 40, 0, 0))
            os.kill(pid, signal.SIGWINCH)
            time.sleep(.2)
            os.write(master, b'sw')
            fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 120, 0, 0))
            os.kill(pid, signal.SIGWINCH)
            time.sleep(.2)
            os.write(master, b'\x1b')
            time.sleep(.15)
            os.write(master, b'q')
            pump_until(b'\x1b[?1049l')
            _, status = os.waitpid(pid, 0)
            reaped = True
            assert os.waitstatus_to_exitcode(status) == 0
            assert b'\x1b[?25h' in output
            after = termios.tcgetattr(master)
            assert after[3] & termios.ICANON, 'canonical mode not restored'
            assert after[3] & termios.ECHO, 'echo not restored'
            assert not before[3] & termios.ICANON, 'test never entered raw mode'
            state = json.loads(command('status', '--id', 'work-4', '--json'))
            assert state['tasks'][0]['state'] == 'done'
            assert state['tasks'][1]['state'] == 'running'
            assert state['revision'] == 2, 'viewer must not write state'
        finally:
            if not reaped:
                try:
                    os.kill(pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                os.waitpid(pid, 0)
            os.close(master)

        # Ctrl+C and external TERM also restore the terminal and do not mutate records.
        for key, sig, code in ((b'\x03', None, 130), (None, signal.SIGTERM, 143)):
            pid, master = pty.fork()
            if pid == 0:
                os.execve(BINARY, [BINARY, 'status', '--id', 'work-4'], env)
            output = bytearray()
            reaped = False
            try:
                pump_until('检查实现'.encode())
                if sig:
                    os.kill(pid, sig)
                else:
                    os.write(master, key)
                pump_until(b'\x1b[?1049l')
                _, status = os.waitpid(pid, 0)
                reaped = True
                assert os.waitstatus_to_exitcode(status) == code
                assert termios.tcgetattr(master)[3] & termios.ICANON
            finally:
                if not reaped:
                    try:
                        os.kill(pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
                    os.waitpid(pid, 0)
                os.close(master)
        print('PTY OK: recent five, W/S/arrows/Enter, live updates, resize, read-only state, Q/Ctrl+C/TERM restore')


if __name__ == '__main__':
    main()
