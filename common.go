package httpcache

const (
	ProxyPresentErrorMessage = "Proxy is present"
)

type (
	ProxyPresentError struct{}
)

func (customErr ProxyPresentError) Error() (res string) {
	res = ProxyPresentErrorMessage
	return
}
