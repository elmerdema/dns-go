import ipaddress
import socket
import struct
import subprocess
import sys
import shutil
import tempfile
import time
from pathlib import Path

HOST = "127.0.0.1"
PORT = 8503
QUERY = sys.argv[1] if len(sys.argv) > 1 else "go.dev"
PROJECT_DIR = Path(__file__).resolve().parent


def encode_name(name):
    labels = name.rstrip(".").split(".")
    return b"".join(bytes([len(label)]) + label.encode() for label in labels) + b"\0"


def skip_name(packet, offset):
    while True:
        length = packet[offset]
        if length == 0:
            return offset + 1
        if length & 0xC0 == 0xC0:
            return offset + 2
        offset += length + 1


def read_ipv4_answers(packet):
    _, flags, questions, answers, _, _ = struct.unpack("!6H", packet[:12])
    offset = 12

    for _ in range(questions):
        offset = skip_name(packet, offset) + 4

    addresses = []
    for _ in range(answers):
        offset = skip_name(packet, offset)
        record_type, _, _, data_length = struct.unpack(
            "!HHIH", packet[offset : offset + 10]
        )
        offset += 10
        data = packet[offset : offset + data_length]
        offset += data_length
        if record_type == 1 and data_length == 4:
            addresses.append(str(ipaddress.ip_address(data)))

    return flags & 0x000F, addresses


query_id = 0x1234
query = (
    struct.pack("!6H", query_id, 0x0100, 1, 0, 0, 0)
    + encode_name(QUERY)
    + struct.pack("!2H", 1, 1)
)

server = None
build_dir = tempfile.mkdtemp(prefix="dns-go-test-")

try:
    executable = Path(build_dir) / "dns-go-test.exe"
    subprocess.run(
        ["go", "build", "-o", str(executable), "."],
        cwd=PROJECT_DIR,
        check=True,
    )
    server = subprocess.Popen([str(executable)], cwd=PROJECT_DIR)

    time.sleep(1.5)
    if server.poll() is not None:
        raise RuntimeError(f"Go server exited with code {server.returncode}")
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as client:
        client.settimeout(3)
        client.sendto(query, (HOST, PORT))
        response, _ = client.recvfrom(4096)

    status, addresses = read_ipv4_answers(response)
    print(f"Query: {QUERY}")
    print(f"Response code: {status}")
    if addresses:
        for address in addresses:
            print(f"Answer: {address}")
    else:
        print("No IPv4 answer found")
except KeyboardInterrupt:
    print("Stopping server...")
finally:
    if server.poll() is None:
        server.terminate()
        server.wait()
    shutil.rmtree(build_dir, ignore_errors=True)
