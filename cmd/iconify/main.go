package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"os"
)

func main() {
	in := flag.String("in", "icon.png", "input PNG path")
	out := flag.String("out", "icon.ico", "output ICO path")
	flag.Parse()

	if err := convertPNGToICO(*in, *out); err != nil {
		fmt.Fprintf(os.Stderr, "iconify: %v\n", err)
		os.Exit(1)
	}
}

func convertPNGToICO(inPath, outPath string) error {
	pngData, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inPath, err)
	}

	conf, _, err := image.DecodeConfig(bytes.NewReader(pngData))
	if err != nil {
		return fmt.Errorf("decode png config: %w", err)
	}

	if conf.Width <= 0 || conf.Height <= 0 {
		return errors.New("invalid image dimensions")
	}

	if conf.Width > 256 || conf.Height > 256 {
		return fmt.Errorf("icon dimensions must be <= 256x256, got %dx%d", conf.Width, conf.Height)
	}

	width := byte(conf.Width)
	height := byte(conf.Height)
	if conf.Width == 256 {
		width = 0
	}
	if conf.Height == 256 {
		height = 0
	}

	buf := bytes.NewBuffer(make([]byte, 0, len(pngData)+64))

	// ICONDIR
	if err := binary.Write(buf, binary.LittleEndian, uint16(0)); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}

	// ICONDIRENTRY
	if err := buf.WriteByte(width); err != nil {
		return err
	}
	if err := buf.WriteByte(height); err != nil {
		return err
	}
	if err := buf.WriteByte(0); err != nil {
		return err
	}
	if err := buf.WriteByte(0); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(32)); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(pngData))); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint32(22)); err != nil {
		return err
	}

	if _, err := buf.Write(pngData); err != nil {
		return err
	}

	if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	return nil
}
