import http from 'k6/http';


export const options = {
    scenarios: {
        contacts: {
            executor: 'constant-vus',
            vus: 500,
            duration: '10s',
        },
    },
};

export default function () {
    const url = 'http://localhost:8001/spot/orders?a=b';

    const payload = JSON.stringify({
        "market": "BTC_USDT",
        "base": "BTC",
        "quote": "USDT",
        "type": "limit",
        "price": "25000",
        "size": "0.0001",
        "side": "sell",
        "user_id": 1,
        "text": "你好世界"
    });

    const params = {
        headers: {
            'Connection': 'Keep-Alive',
            'Content-Type': 'application/json',
            'X-User-ID': '1'
        },
        timeout: '1s'
    };

    http.post(url, payload, params);
}