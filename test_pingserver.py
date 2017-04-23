import socket
import sys

if __name__ == '__main__':
    if len(sys.argv) < 3:
        print("./{} addr port".format(sys.argv[0]))
        sys.exit(1)
    addr = (sys.argv[1], int(sys.argv[2]))
    print(addr)
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.connect(addr)
    sock.send(b'8.8.8.8\n')
    print(sock.recv(1000))
    sock.close()
    sys.exit(0)
