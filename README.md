# Bifrost

一個用 Golang 寫出來的 API 網關函式庫, 可以內嵌在 Golang 程序中, 對性能與延遲敏感的項目友善

## 動機

1. lua 生態慘, 很多第三方服務支持不完善
1. 團隊 Golang 比 lua 熟悉, Golang 套件與積累多
1. 高性能與低延遲特性

## Features

1. 支持用 `Go` 寫你的中間件 (entry, route, service 都可以使用很彈性)
1. 高性能與低延遲特性 (底層用的 HTTP 框架是使用字節開源的 hertz 框架, 經過大廠驗證)
1. 毫秒級熱更新，請求不中斷
1. 支持加密 HTTP2，同時也支持 H2C
1. 原生支持 prometheus metric, opentelemetry 整合

## Roadmap

1. support proxy protcol
1. grpc load balancer

## Credit

1. [CloudWeGo](https://www.cloudwego.io/)
