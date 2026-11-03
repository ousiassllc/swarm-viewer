package compose

import (
	"errors"

	"github.com/ousiassllc/swarm-viewer/internal/repository/store"
	"github.com/ousiassllc/swarm-viewer/internal/router"
)

// Classify は store.Open の失敗を recovery モードの理由に写像する。
//
// 新しすぎるスキーマは RecoverySchemaTooNew、それ以外の DB の問題（壊れている・開けない）は RecoveryCorrupt。
// DB に起因しないエラー（ハンドラ構築の失敗など）は ok=false で、起動の失敗として扱う。
func Classify(err error) (router.RecoveryReason, bool) {
	switch {
	case errors.Is(err, store.ErrSchemaTooNew):
		return router.RecoverySchemaTooNew, true
	case errors.Is(err, store.ErrCorrupt):
		return router.RecoveryCorrupt, true
	default:
		return "", false
	}
}
