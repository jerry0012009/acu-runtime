package main

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// Electron's PE version omits WorkBuddy's build. Read only bounded version
// metadata from its installed archive, without launching a second client.
func workBuddyBuildMetadata(directory string) (string, string) {
	file, err := os.Open(filepath.Join(directory, "resources", "app.asar"))
	if err != nil {
		return "", ""
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", ""
	}
	prefix := make([]byte, 16)
	if _, err = io.ReadFull(file, prefix); err != nil {
		return "", ""
	}
	headerSize := int64(binary.LittleEndian.Uint32(prefix[4:8]))
	jsonSize := int64(binary.LittleEndian.Uint32(prefix[12:16]))
	if binary.LittleEndian.Uint32(prefix[:4]) != 4 || jsonSize <= 0 || jsonSize > 8<<20 || headerSize < jsonSize+8 {
		return "", ""
	}
	type entry struct {
		Files    map[string]json.RawMessage `json:"files"`
		Size     int64                      `json:"size"`
		Offset   string                     `json:"offset"`
		Unpacked bool                       `json:"unpacked"`
	}
	var header entry
	b := make([]byte, jsonSize)
	if _, err = io.ReadFull(file, b); err != nil || json.Unmarshal(b, &header) != nil {
		return "", ""
	}
	read := func(raw json.RawMessage, limit int64) []byte {
		var e entry
		if json.Unmarshal(raw, &e) != nil || e.Unpacked || e.Size <= 0 || e.Size > limit {
			return nil
		}
		offset, err := strconv.ParseInt(e.Offset, 10, 64)
		start := 8 + headerSize + offset
		if err != nil || offset < 0 || start < 0 || start+e.Size > info.Size() {
			return nil
		}
		data := make([]byte, e.Size)
		if _, err = file.ReadAt(data, start); err != nil {
			return nil
		}
		return data
	}
	var pkg struct {
		Version string `json:"version"`
	}
	json.Unmarshal(read(header.Files["package.json"], 64<<10), &pkg)
	var main entry
	json.Unmarshal(header.Files["main"], &main)
	data := read(main.Files["main-bootstrap.js"], 4<<20)
	match := regexp.MustCompile("BUILD_COMMIT\\s*=\\s*[`\"']([a-fA-F0-9]{8,40})[`\"']").FindSubmatch(data)
	if len(match) != 2 {
		return pkg.Version, ""
	}
	return pkg.Version, string(match[1])
}
