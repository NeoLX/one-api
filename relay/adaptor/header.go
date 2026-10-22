package adaptor

import "net/http"

// 这些头不允许被渠道自定义，会破坏请求本身
var customHeaderDenyList = map[string]bool{
	"Host":            true,
	"Content-Length":  true,
	"Connection":      true,
	"Accept-Encoding": true,
}

// ApplyCustomHeaders 把渠道配置里的自定义请求头写进 h，覆盖同名头
// （包括 Authorization；同名头原有的多个值会被替换为配置里的单个值）。
// 空键名、黑名单内的头会被跳过。h 必须非 nil。
func ApplyCustomHeaders(h http.Header, headers map[string]string) {
	for k, v := range headers {
		if k == "" || customHeaderDenyList[http.CanonicalHeaderKey(k)] {
			continue
		}
		h.Set(k, v)
	}
}
