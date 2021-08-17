package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/pierrec/lz4"
)

func lz4Compress(ctx context.Context, inputFile, outputFile string) (string, error) {

	id := getReqIDFromContext(ctx)

	var blockMaxSizeDefault = 4 << 20

	fmt.Println(blockMaxSizeDefault)

	zw := lz4.NewWriter(nil)
	zh := lz4.Header{
		BlockDependency: false,
		BlockChecksum:   false,
		BlockMaxSize:    blockMaxSizeDefault,
		NoChecksum:      false,
		HighCompression: false,
	}

	in, err := os.Open(inputFile)
	if err != nil {
		fail.Printf("%sfailed to open %s: %v\n", id, inputFile, err)
		return "", err
	}

	defer func() {
		id := getReqIDFromContext(ctx)
		err = in.Close()
		if err != nil {
			fail.Printf("%sfailed in defer: %s", id, err.Error())
		}
	}()

	info.Printf("%sopened: %s\n", id, inputFile)

	out, err := os.Create(outputFile)
	if err != nil {
		fail.Printf("%sfailed to open %s: %v\n", id, outputFile, err)
		return "", err
	}

	defer func() {
		id := getReqIDFromContext(ctx)
		err = out.Close()
		if err != nil {
			fail.Printf("%sfailed in defer: %s", id, err.Error())
		}
	}()

	info.Printf("%sopened: %s\n", id, outputFile)

	zw.Reset(out)
	zw.Header = zh

	_, err = io.Copy(zw, in)
	if err != nil {
		fail.Printf("%sfailed to compress %s: %v\n", id, inputFile, err)
		return "", err
	}
	info.Printf("%scompressed: %s\n", id, inputFile)

	err = zw.Close()
	if err != nil {
		fail.Printf("%sfailed to close stream: %v\n", id, err)
		return "", err
	}
	info.Printf("%sclosed stream: %s\n", id, outputFile)

	return outputFile, err
}
