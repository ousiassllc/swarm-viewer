package handler

import (
	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"

	"github.com/ousiassllc/swarm-viewer/internal/web/layout"
)

// responder は共通の応答処理（base）を panehandler.Responder として公開する薄いアダプタ。
type responder struct{ base }

func (r responder) Render(c *gin.Context, status int, comp templ.Component) {
	r.render(c, status, comp)
}

func (r responder) View(c *gin.Context, title string, page func(layout.BaseProps) templ.Component, content templ.Component) {
	r.view(c, title, page, content)
}

func (r responder) Fail(c *gin.Context, status int, message string) { r.fail(c, status, message) }

func (r responder) NotFound(c *gin.Context) { r.notFound(c) }

func (r responder) Internal(c *gin.Context, what string, err error) { r.internal(c, what, err) }
