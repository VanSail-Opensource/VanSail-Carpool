## 概述

WHMCS 可用的 BEpusdt 支付网关，开发测试可用的 whmcs 版本是 8.10.1，其它版本没测试过，有问题请到 BEpusdt issue 反馈。

## 目录结构

```
whcms-gateway-epusdt/
 | epusdt.php
 │
 |-callback
 │      epusdt_notify.php
 │      epusdt_return.php
 │
 └─epusdt
     │  whmcs.json
     │
     └─lib
        epusdt.php
```

## 对接

解压后文件复制到 `modules/gateways` 目录下即可

## 声明

```
仅用于学习交流使用
作者仅完成代码的开发和开源活动(开源即任何人都可以下载使用或修改分发)，从未参与用户的任何运营和盈利活动。
且不知晓用户后续将程序源代码用于何种用途，故用户使用过程中所带来的任何法律责任即由用户自己承担。
```
