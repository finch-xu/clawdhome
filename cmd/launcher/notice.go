package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/finch-xu/clawdhome/vm"
)

type notice struct {
	key    string
	args   []any
	detail string
}

func (n notice) text(loc locale) string {
	text := tr(n.key, loc)
	if len(n.args) != 0 {
		return fmt.Sprintf(text, n.args...)
	}
	return text
}

func errorNotice(fallback string, err error) notice {
	if err == nil {
		return notice{}
	}
	key := fallback
	switch code := vm.ErrorCode(err); code {
	case "":
	case vm.ErrPortRange:
		key = "settings.portRange"
	default:
		key = "error." + code
	}
	return notice{key: key, detail: err.Error()}
}

func (a *app) noticeView(c *ui.Context, n notice) ui.Element {
	text := ui.Text(c, n.text(a.locale)).SingleLine().Ellipsis("…")
	if n.detail != "" {
		text.Tooltip(n.detail)
	}
	return text
}
