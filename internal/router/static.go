package router

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const staticPrefix = "/static"

// registerStatic は埋め込みアセットを `/static/*` で配信する。ディレクトリの一覧は出さない。
func registerStatic(e *gin.Engine, assets fs.FS) error {
	if assets == nil {
		return errors.New("router: static assets are required")
	}
	serve := func(c *gin.Context) {
		name := strings.TrimPrefix(path.Clean("/"+c.Param("filepath")), "/")
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		// 本文は埋め込みの固定アセット。Content-Type は拡張子から決まる。
		http.ServeContent(c.Writer, c.Request, name, time.Time{}, bytes.NewReader(data))
	}
	e.GET(staticPrefix+"/*filepath", serve)
	e.HEAD(staticPrefix+"/*filepath", serve)
	return nil
}
