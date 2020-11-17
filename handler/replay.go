package handler

import (
	"fmt"
	"io"
	"os"
)

// Replay .
func Replay(ctx *Context) error {
	defer ctx.Close()
	fileName := ParamGet(ctx, "file")
	charset := fixCharset(ParamGet(ctx, "charset"))
	dumpOut, err := os.Open(fileName)
	if nil != err {
		return fmt.Errorf("open '"+fileName+"' failed: %w", err)
	}
	defer dumpOut.Close()

	if _, err := io.Copy(decodeBy(charset, ctx), dumpOut); err != nil {
		return fmt.Errorf("copy of stdout failed: %w", err)
	}
	return nil
}
