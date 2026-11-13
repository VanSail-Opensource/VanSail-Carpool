<?php

use gateways\epusdt\lib\Epusdt;

require_once __DIR__ . '/../../../init.php';
require_once __DIR__ . '/../../../includes/gatewayfunctions.php';
require_once __DIR__ . '/../../../includes/invoicefunctions.php';

// 修改类检查和引入
if (!class_exists('gateways\epusdt\lib\Epusdt')) {
    require_once __DIR__ . '/../epusdt/lib/epusdt.php';
}

$gatewayConfig = getGatewayVariables('epusdt');
$payload       = json_decode(file_get_contents("php://input"), true);

if ($payload["status"] == 2) {
    $epusdt = new Epusdt($gatewayConfig['endpoint'], $gatewayConfig['secret']);

    if ($epusdt->verifySignature($payload['signature'], $payload)) {
        $invoiceId = checkCbInvoiceID($payload['order_id'], 'epusdt');
        checkCbTransID($payload['trade_id']);
        logTransaction('epusdt', $payload, 'Success');
        addInvoicePayment($invoiceId, $payload['trade_id'], $payload['money'], 0, 'epusdt');

        exit('ok');
    }
}

exit('fail');