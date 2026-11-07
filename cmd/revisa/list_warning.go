package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/vieitesss/revisa.nvim/internal/project"
)

func warnLocalComments(p project.Project, stderr io.Writer) error {
	if !p.Git || p.Root == "" {
		return nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	pending, err := p.PendingLocalStores(wd)
	for _, local := range pending {
		if local.Err != nil {
			name := filepath.ToSlash(filepath.Join(local.Dir, ".revisa"))
			fmt.Fprintf(stderr, "cannot check %s/: %v\n", name, local.Err)
			continue
		}
		fmt.Fprintf(stderr, ".revisa/ comments found "+
			"in %s; run revisa migrate to move them "+
			"into the git store\n", local.Dir)
	}
	return err
}
