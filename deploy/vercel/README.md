# Vercel proxy

このdirectoryをVercel projectのRoot Directoryにする。

環境変数`CLOUD_RUN_ORIGIN`へCloud Runのservice URLを末尾の`/`なしで設定する。`vercel.json`は全requestをそのoriginへ転送し、Vercel側のcacheを無効にする。

Vercel FunctionやMiddlewareは使わない。
