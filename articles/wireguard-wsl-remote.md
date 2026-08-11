---
title: "VPSとVPNルーターで実現する、外出先からのWSLリモート接続【WireGuard】"
emoji: "🐉"
type: "tech"
topics: ["ssh", "wsl2", "vpn", "さくらvps", "wireguard"]
published: true
published_at: 2025-06-29 04:50
---
## 2026/4/3 追記
しばらく前にTailscaleに移行してからこの構成は使用していません
でも結果的にTailscaleのベースとなっているWireguardについて学べたのは良かったです

## やったこと
自宅のVPNルータと持ち運び用デバイスをwireguradクライアントとして設定。
さくらVPSでレンタルした最小スペックのDebianに、wireguardサーバを建ててクライアントと接続。
自宅LAN内でwslのkali linuxにsshをできるように設定。

## 使ったもの
### TP-Link Omada ER605 (VPNルーター)
https://www.amazon.co.jp/TP-Link-Omada-%E3%82%AE%E3%82%AC%E3%83%93%E3%83%83%E3%83%88-%E3%83%9E%E3%83%AB%E3%83%81WAN-VPN%E3%83%AB%E3%83%BC%E3%82%BF%E3%83%BC/dp/B08MH4VLR3/ref=nav_signin?mcid=30f978e042b53a838cc559e2e9716ad4&tag=jpgo-22&linkCode=df0&hvadid=707549940401&hvpos=&hvnetw=g&hvrand=7323411371706617902&hvpone=&hvptwo=&hvqmt=&hvdev=c&hvdvcmdl=&hvlocint=&hvlocphy=9197728&hvtargid=pla-1063525511229&psc=1&hvocijid=7323411371706617902-B08MH4VLR3-&hvexpln=0

### さくらVPS
Debian 12 amd64 大阪第3 512MB
https://vps.sakura.ad.jp/specification/

### WSL2
KaliLinux(wsl) SSHの接続先

## 手順
### Configの生成
wireguardの設定ファイルを生成してくれるサイトがあります。genkey等で鍵を生成する手間が省けるので便利です。
https://www.wireguardconfig.com/
ここでGenerate Configを行う前に設定する値は、**Number of Clients**と**Client Allowed IPs**と**Endpoint**のみです。

**Number of Clients**は接続するクライアントの数を入力してください。
私はスマホ等いろんなデバイスを追加したので、現在のクライアント数は6つくらいになっていますが、最低限VPNルータ用とデバイス用があればいいので`2`でも大丈夫です。
peerを自分で追記すれば、後からクライアントを追加することも可能です。

**Client Allowed IPs**は、クライアント側が通信を行う際にどのipアドレス向けのパケットをwireguardに通すかを設定する部分です。
デフォルトの`0.0.0.0/0, ::/0`では全てのipv4, ipv6の通信をwireguard越しに行います。

この状態でDNSに8.8.8.8などを設定すると完全なVPNが構築できるのですが、私が欲しかったのは自宅のwslへのsshのみで、他の通信は秘匿する必要がないので、ここには`10.0.0.0/24, 192.168.0.0/24`を設定しました。

**Endpoint**は、レンタルしたVPSのipアドレスまたはホスト名の51820番ポートを入力してください。
(例: `******.vs.sakura.ne.jp:51820`)

Generate Configを押すと、下にServer用とClient用の設定ファイルが生成されました。これらをそれぞれVPSとデバイス/ルータに設定していきます。

このページで生成した情報をこの後何度か参照するので、zipのダウンロード等行った上で、ページを閉じないでおくことをお勧めします。

### サーバ側の設定
AllowedIPs = 10.0.0.0/24, 192.168.0.0/24

さくらVPSでレンタルしたdebianにssh接続し、wireguardをインストールします。
適宜aptのアップデート等も行ってください。
```
$ sudo apt install wireguard
```

インストール後、etc以下にwireguard/wg0.confを作成して編集。
```
$ sudo vi /etc/wireguard/wg0.conf
```

基本的には先ほど作成したサーバ用設定をそのまま記述すれば良いです。
ただし、外出先からLANにアクセスできるように、VPNルーター用のPeerのAllowedIPsには`192.168.0.0/24`を追加します。
これによって、他のピアから送られてきた`192.168.0.0/24`向けのパケットは全てルーターに流されます。

```
[Interface]
Address = 10.0.0.1/24
ListenPort = 51820
PrivateKey = *****
PostUp = iptables -A FORWARD -i %i -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE


[Peer]
PublicKey = *****
AllowedIPs = 10.0.0.2/32

[Peer]
PublicKey = *****
AllowedIPs = 10.0.0.3/32, 192.168.0.0/24

...
```
私は2番目のPeer(つまり、生成した設定ファイルでいうところのClient2)をルーター用に決めました。

設定後、wireguardを起動します
```
$ sudo systemctl enable wg-quick@wg0
$ sudo systemctl start wg-quick@wg0
```

起動後、設定変更した時は`$ sudo systemctl restart wg-quick@wg0`等で再起動が必要です。私はこれを忘れて少し詰まりました。

実行中は
```
$ sudo wg
```
で設定を確認できます。

### クライアント側の設定

#### ER605 (VPNルーター)
このルーターのLAN内で`http://192.168.0.1`にアクセスすると、ルーターの設定ページが開けます。

VPNの項目を開いて、wireguardの設定を行います。

ルーター用として決めたClient設定ファイルを用意して、設定を書き写していきます。

wireguardの方には、設定ファイルの[Interface]部分を記入してください。
![](/images/7911e9353b8e-20250629.png)

MTUは転送できるパケットのサイズなのでデフォルトのままで大丈夫です。Nameは自分でわかりやすい名前を決めてください

Peerには設定ファイルの[Peer]部分を記入してください
![](/images/35b0bdc3bb40-20250629.png)

ここで、トンネル越しにルーターへ送られてくるパケットの送信元は全て`10.0.0.0/24`に含まれており、`192.168.0.0/24`宛のパケットを外部に送る必要もないので、Allowed Adressは`10.0.0.0/24`のみで大丈夫です。

AllowedIPsについてはこちらの記事がわかりやすかったです
https://www.infrastudy.com/?p=1065

#### デバイス
私はmacにGUIクライアントツールを入れました。各種OSに対応しています。
https://www.wireguard.com/install/

また、iOS、Android向けクライアントアプリもアプリストアから取得できます。

「トンネルを追加」から設定内容のコピペを行うか、設定ファイルのインポートで簡単に設定できます。

モバイル向けアプリではQRコードの読み込みでも設定できるはずです。

### ssh
ここまでの内容で、`192.168.0.0/24`へのアクセスがトンネル越しにルーターまで伝わるようになりました。

ここから、wslでインストールしたkali linuxにssh接続を行えるようにする...のですが、具体的にwslにsshする方法はたくさんの方が書かれているのでそちらにお任せします。投げやりですみません

この辺りの記事を参考にした覚えがあります。(曖昧)
https://qiita.com/yuta-katayama-23/items/fad6928f37badf3391f2

## おまけ
スマホからssh接続を行うとき、いくつかのsshクライアントアプリを入れて試してみたのですが、Termiusが一番使いやすかったです。
https://termius.com/index.html

無料でも十分使える上に、Github Education Packに登録していると、学生は無料でProプラン相当の機能を使えます。おすすめです

---
この記事は、先月自分が行ったセットアップをconfigと記憶を頼りに思い出しながら書いたものなので、誤りがあるかもしれません。
誤りがあった場合はコメントで教えてください。
