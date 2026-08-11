---
title: "Kea DHCPのIPアドレスリース数のメトリクス取得が難しかった話"
emoji: "🥞"
type: "tech"
topics: ["dhcp", "janog", "kea", "stork"]
published: true
published_at: 2026-04-08 12:46
---
2026年2月に行われたJANOG57でNOCに参加し、ServerチームでDHCPサーバの構築をしていました。
DHCPサーバの構築は全てさくらのクラウド上で行いました。さくらのクラウド上でのインフラ構築はterraformとansibleを用いて自動化しており、実際に使用していたソースのリポジトリはイベント終了後公開されています。
https://github.com/janog57-noc/janog57-infra

構成としてはKea DHCPサーバ2台とその管理を行うStorkサーバ1台という運用をしていました。

Kea、Storkは共にISC(Internet Systems Consortium)が公開しているOSSで、KeaはISC DHCPの後継となるDHCPソフトウェア、StorkはISCの公開するDHCP(Kea)やDNS(BIND)の管理を行うソフトウェアです。

主に行なった作業内容としては、このKeaとStorkのサーバ構築が中心ですが、サーバ本体以外にも、NOCで使用していたNetboxというインフラ管理ツールから、会場のAPのmacアドレスと固定ipアドレスの対応を抜き出してansible playbook向けにymlで出力するスクリプトを書いたり、stork serverのAPIから詳細なメトリクスを取得して公開するPrometheus exporterを作ったりしました。

https://github.com/janog57-noc/ap_mac_ip_mapping
https://github.com/janog57-noc/stork-subnet-exporter

# Kea DHCP, Kea CA, Stork Server, Stork Agentについて
![](/images/1ca1112d5518-20260402.png)
> Serverチームで学生リーダーを務めてくださったcrashRTさんの記事から画像を引用しています
> https://blog.crashrt.work/entry/2026/03/03/144548#DHCP%E3%81%AE%E6%A7%8B%E6%88%90

まず、KeaとStorkの関係を繋ぐ要素の一つとして、Kea DHCPの情報をREST APIで公開するKea CA(Control Agent)があります。これはKea DHCP本体とは独立して起動するデーモンです。

:::message
なお、今回使用したKeaはver 3.0.2だったので、本来はKea CAを用いなくても、Kea DHCP単体でREST APIを公開できるようになっています。公式のガイドでも、Kea 2.7.2以降Kea CAの使用が非推奨になった旨が書かれています。
https://kea.readthedocs.io/en/stable/arm/agent.html

しかし、NOCの活動期間中の時点ではStork側がまだCAを介さない構成に対応していなかったので、今回の構成ではKea CAを用いて構築しました。

2026年2月25日にリリースされたStork 2.4ではCAを介さないAPIアクセスに対応したので、今後はKea CAを使用する必要がなくなるはずです。
https://www.isc.org/blogs/stork-2-4/
:::

このように、Keaが持つリース情報などを取得するAPIを介して、StorkはDHCPの管理を行います。この時に、中央で管理を行うStork Serverと、KeaからREST APIで情報を取得し、gRPCでServerとやりとりする、Stork Agentという二つの構成要素があります。

Stork Serverは各DHCPから得た情報をPostgreSQLのDBに保存し、状態の監視やhookを用いたKeaの操作などを行うことができるWeb UIをホスティングします。また、Prometheus向けの`/metrics`エンドポイントや、認証が必要な`/api`エンドポイントなども提供します。

基本的には、Prometheusでのメトリクス収集はこの`/metrics`を用いるように設計されているのですが、`/metrics`エンドポイントから得られる情報は少なく、リース数に関する情報はサブネットごとの使用率くらいしか取得できません。

例えば、私の家で使用しているKea DHCPサーバと共存しているStork Serverの`/metrics`にGETリクエストを送ると、返ってくる情報はこれだけです。

```
# HELP storkserver_auth_authorized_machine_total Authorized machines
# TYPE storkserver_auth_authorized_machine_total gauge
storkserver_auth_authorized_machine_total 1
# HELP storkserver_auth_unauthorized_machine_total Unauthorized machines
# TYPE storkserver_auth_unauthorized_machine_total gauge
storkserver_auth_unauthorized_machine_total 0
# HELP storkserver_auth_unreachable_machine_total Unreachable machines
# TYPE storkserver_auth_unreachable_machine_total gauge
storkserver_auth_unreachable_machine_total 0
# HELP storkserver_subnet_address_utilization Subnet address utilization
# TYPE storkserver_subnet_address_utilization gauge
storkserver_subnet_address_utilization{name="",subnet="192.168.0.0/24"} 0.025
# HELP storkserver_subnet_pd_utilization Subnet delegated-prefix utilization
# TYPE storkserver_subnet_pd_utilization gauge
storkserver_subnet_pd_utilization{name="",subnet="192.168.0.0/24"} 0

```

今回のNOCでは、2台のDHCPサーバの会場ごとの具体的なリース数などをGrafana Dashboardに表示させたかったので、この情報だけでは不十分でした。

この問題についてはcrashRTさんが事前に把握していたので、その対処法として、各DHCPサーバと同居するStork Agentの`/metrics`エンドポイントを使用するという手段を教えてもらいました。こっちのエンドポイントからは、`kea_dhcp4_addresses_assigned_total`(現在のサブネットごとのリース数)や`kea_dhcp4_addresses_total`(サブネットごとの割り当て可能なアドレス数)などの情報が得られます。

当初はこれで問題なくPrometheus/Grafanaにリース数メトリクスが提供できるはず、という認識でした。

# assigned-addresses: 18446744073709551600

しばらく起動して動作確認していると、一部のサブネットで明らかにおかしい数のリース数がGrafanaに表示されるようになってしまいました。
![](/images/ac766b2738ff-20260403.png)

概ね2^64のような値であったことから、負の値が誤って解釈されているのではないかと考察しました。実際、KeaのAPIを直接叩いてみたところ、確かにリース数がマイナスになっていることが原因であることが確認できました。

なぜKeaがリース数としてマイナスの値を返しているのでしょうか。

この問題については、二つの要因があると思われます
一つは、恐らく今回使用したJunosのrelay agentが、DHCPサーバーに宛てのユニキャストで送信されるDHCPRELEASEもキャッチしてrelayするような実装になっていたこと。
もう一つは、二重に到着したDHCPRELEASEをKeaが二度カウントしてしまっていたことです。

RFCではDHCP relayの仕様がふんわりとしか決まっていないため、様々な実装が存在します。Junosの他にも、VyOSも似た挙動をしました。relay agentがブロードキャストのパケットであるかどうかに関わらずリレーしようとするせいで、本来のユニキャストとして送信されたパケットとrelay agentによって再生成されたパケットの二つがKea DHCPに到着してしまいます。

そして、Kea DHCPでは到着したDHCPREQUESTとDHCPRELEASEの数でassigned addressesの値を増減させています。
https://kea.readthedocs.io/en/latest/arm/dhcp4-srv.html#dhcp4-stats
> Number of assigned addresses in a given subnet. It increases every time a new lease is allocated (as a result of receiving a DHCPREQUEST message) and decreases every time a lease is released (a DHCPRELEASE message is received) or expires.

ここで、一つ目のDHCPRELEASEの後に、解放済みのreclaimedなリースに関する二つ目のDHCPRELEASEを送信するスクリプトを書いて実験してみたところ、二重にカウントが減算されることが明らかになりました。

```python
from scapy.all import *

### (略)

# 1. 正規のDHCPRELEASE（クライアントから直接）
normal_release = (
    IP(src=CLIENT_IP, dst=SERVER_IP) /
    UDP(sport=68, dport=67) /
    BOOTP(
        op=1, htype=1, hlen=6, hops=0,
        xid=RandInt(),
        ciaddr=CLIENT_IP,
        giaddr="0.0.0.0",
        chaddr=mac2str(CLIENT_MAC) + b'\x00'*10,
    ) /
    DHCP(options=[
        ("message-type", "release"),
        ("server_id", SERVER_IP),
        "end"
    ])
)

# 2. relay経由のDHCPRELEASE（同じリースに対して）
relay_release = (
    IP(src=GIADDR, dst=SERVER_IP) /
    UDP(sport=67, dport=67) /
    BOOTP(
        op=1, htype=1, hlen=6, hops=1,
        xid=RandInt(),
        ciaddr=CLIENT_IP,
        giaddr=GIADDR,
        chaddr=mac2str(CLIENT_MAC) + b'\x00'*10,
    ) /
    DHCP(options=[
        ("message-type", "release"),
        ("server_id", SERVER_IP),
        "end"
    ])
)

send(normal_release, verbose=True)
send(relay_release, verbose=True)
```

結果

```json
{
    "arguments": {
        "subnet[1].assigned-addresses": [
            [
                2,　// <- 同じリースに対するDHCPRELEASEでカウントが2減っている
                "2026-04-05 03:41:56.458583"
            ],
            [
                3,
                "2026-04-05 03:41:54.370551"
            ],
            // ---------スクリプト実行開始---------
            [
                4,
                "2026-04-05 03:36:51.858761"
            ],
            [
                // (略)
```

さらに、環境の再現のためにsportやsource IPをちゃんと別にしてパケットを送信しましたが、このように完全に同じパケットを複数送信してもそのまま減算されました。

```python
send(normal_release, verbose=True)
send(normal_release, verbose=True)
send(normal_release, verbose=True)
send(relay_release, verbose=True)
send(relay_release, verbose=True)
send(relay_release, verbose=True)
```

```json
{
    "arguments": {
        "subnet[1].assigned-addresses": [
            [
                -3,
                "2026-04-05 03:46:42.776886"
            ],
            [
                -2,
                "2026-04-05 03:46:40.670568"
            ],
            [
                -1,
                "2026-04-05 03:46:38.576405"
            ],
            [
                0,
                "2026-04-05 03:46:36.482428"
            ],
            [
                1,
                "2026-04-05 03:46:34.398569"
            ],
                // (略)
```

色々調査した結果、この問題は以下のような設定で「`pkt4.msgtype == 7`(DHCPRELEASE)の時のみ、ユニキャスト以外のパケットをDROP」というフィルタリングを行うことで回避できることが分かりました。
```
"client-classes": [
    {
        "name": "DROP",
        "test": "pkt4.msgtype == 7 and not (pkt4.giaddr == 0.0.0.0)"
    }
]
```

`client-classes`の詳しい設定使い方はISCのドキュメントを確認してください。
https://kb.isc.org/docs/understanding-client-classification

ここまでの内容は、この記事を書くにあたって追加で調査した結果分かったことであり、会期中にここまで到達することはできませんでした。

JANOG NOCで実際に行った対処は次のようなものです。

# stork-server-exporter

この問題に直面した際、Stork ServerのWeb UIを確認してもグラフやリース数に問題がありそうな箇所がなかったことから、私はKeaの提供するリース数の情報とは別に、Stork Server側でもリース数を計算して表示しているものだと思いました。よって、この値を参照することができれば、Prometheus/Grafanaにも正しいメトリクスを提供できると判断しました。

前述の通り、Stork Serverには簡素な`/metrics`エンドポイントと、認証が必要な`/api`エンドポイントがあります。`/api`エンドポイントはStork ServerのWeb UIが実際に叩いている部分であるため、必然的にWeb UIで得られる情報は全てこのエンドポイントから得られます。
APIではStork ServerのWeb UIと同様のIDとパスワードでログインし、セッションベースの認証を行うため、その部分を自動化し、必要な情報のみServerのAPIから参照してPrometheus向けに公開する[stork-subnet-exporter](https://github.com/janog57-noc/stork-subnet-exporter)を作成しました。

この変更によって変な値が出る問題が解消されたように見えたので、一件落着と安堵していました

---

残念ながら、stork-subnet-exporterを使用するようになっても、問題は再発しました。そして改めてStork ServerのWeb UIを確認していたところ、Stork ServerのWeb UIでは正常な値が表示されているのではなく、ただ単に壊れた値のデータは表示されずグラフも消えていることを見落としていただけでした。
![](/images/59da4efa234f-20260403.png)

実際、Stork Server側でリース数の再計算などは行われておらず、情報のソースとなっているのはKea本体でした。

結局、会期終了までこの問題は解決できませんでした。JANOG57では会場インフラの状態を確認できるGrafanaダッシュボードをYoutubeLiveで配信していたのですが、そのアーカイブでも変な値が出ていたことが確認できます。
https://www.youtube.com/live/rK9xIaFACd4?si=FviYIR7G8K3pBwBk&t=1647

# 結論

会期中にこの問題を解決することはできませんでしたが、前述の`giaddr`で`client-class`を分けるという方法や、DBのリースを確認して都度計算するプログラムを別途用意するといった方法には解決の可能性がありそうです。

トラブルが発生した際に正確なリース数が把握できなかったら非常に大変でしたが、幸い会期中DHCPサーバで大きなトラブルが発生することはなく、イベント終了まで無事に動作させることができました。

また、問題調査や対策にあたって、自分の知識不足や能力不足を感じる場面が多々ありました。stork-subnet-exporterも即席で用意したものなので、今見返すとコード中の無駄な部分などが目につきます。問題や障壁にぶつかる度に、他のNOC先輩の方々や社会人メンバーの方々に助けてもらいました。改めて本当にありがとうございました

# 検証に使用したスクリプト
https://github.com/SouichiroTsujimoto/kea-dhcp-zenn
