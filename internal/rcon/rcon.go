package rcon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const (
	typeExecCommand  int32 = 2
	typeAuth         int32 = 3
	typeAuthResponse int32 = 2
	typeRespValue    int32 = 0
)

func writePacket(w io.Writer, id, ptype int32, body string) error {
	payload := append([]byte(body), 0, 0)
	length := int32(4 + 4 + len(payload))
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, length)
	_ = binary.Write(buf, binary.LittleEndian, id)
	_ = binary.Write(buf, binary.LittleEndian, ptype)
	buf.Write(payload)
	_, err := w.Write(buf.Bytes())
	return err
}

func readPacket(r io.Reader) (int32, int32, string, error) {
	var length int32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return 0, 0, "", err
	}
	if length < 10 || length > 65536 {
		return 0, 0, "", fmt.Errorf("invalid rcon packet size: %d", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return 0, 0, "", err
	}
	id := int32(binary.LittleEndian.Uint32(data[0:4]))
	ptype := int32(binary.LittleEndian.Uint32(data[4:8]))
	body := strings.TrimRight(string(data[8:]), "\x00")
	return id, ptype, body, nil
}

func Exec(addr, password, command string) (string, error) {
	return ExecWithTimeout(addr, password, command, 3*time.Second, 2000*time.Millisecond)
}

func ExecWithTimeout(addr, password, command string, dialTimeout, firstReadWait time.Duration) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", errors.New("RCON 密码未配置")
	}
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return "", fmt.Errorf("无法连接 CS2 RCON (%s): %w", addr, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(dialTimeout + firstReadWait + 2*time.Second))

	if err := writePacket(conn, 1, typeAuth, password); err != nil {
		return "", err
	}
	// Source RCON may send an empty SERVERDATA_RESPONSE_VALUE before SERVERDATA_AUTH_RESPONSE
	authed := false
	for i := 0; i < 3; i++ {
		id, ptype, _, err := readPacket(conn)
		if err != nil {
			return "", fmt.Errorf("RCON 认证读取失败: %w", err)
		}
		if ptype == typeAuthResponse {
			if id == -1 {
				return "", errors.New("RCON 密码认证失败")
			}
			authed = true
			break
		}
	}
	if !authed {
		return "", errors.New("RCON 认证未收到响应")
	}

	if err := writePacket(conn, 2, typeExecCommand, command); err != nil {
		return "", err
	}

	var sb strings.Builder
	_ = conn.SetReadDeadline(time.Now().Add(firstReadWait))
	for {
		id, _, body, err := readPacket(conn)
		if err != nil {
			break
		}
		if id == 2 {
			sb.WriteString(body)
			_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		}
	}
	return strings.TrimSpace(sb.String()), nil
}
