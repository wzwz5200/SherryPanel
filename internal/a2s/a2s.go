package a2s

import (
	"bytes"
	"encoding/binary"
	"net"
	"time"
)

type ServerStatus struct {
	Online     bool   `json:"online"`
	Name       string `json:"name"`
	Map        string `json:"map"`
	Game       string `json:"game"`
	Players    int    `json:"players"`
	Bots       int    `json:"bots"`
	MaxPlayers int    `json:"max_players"`
}

func Query(addr string) ServerStatus {
	st := ServerStatus{Online: false}
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		return st
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	req := append([]byte{0xFF, 0xFF, 0xFF, 0xFF}, []byte("TSource Engine Query\x00")...)
	if _, err := conn.Write(req); err != nil {
		return st
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil || n < 6 {
		return st
	}
	data := buf[:n]
	if n >= 9 && data[4] == 0x41 {
		req2 := append(req, data[5:9]...)
		if _, err := conn.Write(req2); err != nil {
			return st
		}
		n, err = conn.Read(buf)
		if err != nil || n < 6 {
			return st
		}
		data = buf[:n]
	}
	if data[4] != 0x49 {
		return st
	}
	idx := 6
	readStr := func() (string, bool) {
		end := bytes.IndexByte(data[idx:], 0)
		if end < 0 {
			return "", false
		}
		s := string(data[idx : idx+end])
		idx += end + 1
		return s, true
	}
	name, ok1 := readStr()
	mapName, ok2 := readStr()
	_, ok3 := readStr() // folder
	game, ok4 := readStr()
	if !ok1 || !ok2 || !ok3 || !ok4 || idx+5 > len(data) {
		return st
	}
	_ = binary.LittleEndian.Uint16(data[idx : idx+2])
	total := int(data[idx+2])
	maxP := int(data[idx+3])
	bots := int(data[idx+4])
	humans := total - bots
	if humans < 0 {
		humans = 0
	}
	return ServerStatus{
		Online:     true,
		Name:       name,
		Map:        mapName,
		Game:       game,
		Players:    humans,
		Bots:       bots,
		MaxPlayers: maxP,
	}
}
