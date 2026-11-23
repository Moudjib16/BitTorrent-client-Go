// Command bittorrent-client-go downloads a torrent from the command line.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/Moudjib16/BitTorrent-client-Go/torrentfile"
)

func main() {
	// Ctrl+C should stop the download instead of killing the process halfway.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("bittorrent-client-go", flag.ContinueOnError)
	dir := flags.String("dir", ".", "directory to download the torrent into")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: bittorrent-client-go [flags] <torrent file>")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil // the usage was already printed
		}
		return err
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return fmt.Errorf("expected exactly one .torrent file")
	}

	torrent, err := torrentfile.Open(flags.Arg(0))
	if err != nil {
		return err
	}

	fmt.Printf("downloading %s (%s)\n", torrent.Name, humanSize(torrent.Length))

	status := newStatusPrinter()
	err = torrent.DownloadToDir(ctx, *dir, status.update)
	status.finish()
	if err != nil {
		return err
	}

	fmt.Println("done")
	return nil
}

func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
