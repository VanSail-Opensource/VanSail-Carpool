<?php

use gateways\epusdt\lib\Epusdt;

if (!defined('WHMCS')) die('This file cannot be accessed directly');

if (!class_exists('gateways\epusdt\lib\Epusdt')) {
    require __DIR__ . '/epusdt/lib/epusdt.php';
}

function epusdt_MetaData()
{
    return [
        'DisplayName' => 'BEpusdt 一款更好用的个人加密货币收款网关',
    ];
}

function epusdt_config()
{
    return [
        'FriendlyName' => [
            'Type'  => 'System',
            'Value' => 'BEpusdt',
            'Size'  => 128,
        ],
        'trade_type'   => [
            'FriendlyName' => 'Trade Type',
            'Type'         => 'dropdown',
            'Options'      => [
                'usdt.trc20'    => 'USDT - Tron (TRC20)',
                'usdt.erc20'    => 'USDT - Ethereum (ERC20)',
                'usdt.polygon'  => 'USDT - Polygon',
                'usdt.bep20'    => 'USDT - BSC (BEP20)',
                'usdt.aptos'    => 'USDT - Aptos',
                'usdt.solana'   => 'USDT - Solana',
                'usdt.xlayer'   => 'USDT - X-Layer',
                'usdt.arbitrum' => 'USDT - Arbitrum One',
                'usdt.plasma'   => 'USDT - Plasma',
                'usdc.trc20'    => 'USDC - Tron (TRC20)',
                'usdc.erc20'    => 'USDC - Ethereum (ERC20)',
                'usdc.polygon'  => 'USDC - Polygon',
                'usdc.bep20'    => 'USDC - BSC (BEP20)',
                'usdc.aptos'    => 'USDC - Aptos',
                'usdc.solana'   => 'USDC - Solana',
                'usdc.xlayer'   => 'USDC - X-Layer',
                'usdc.arbitrum' => 'USDC - Arbitrum One',
                'usdc.base'     => 'USDC - Base',
                'tron.trx'      => 'TRX - Tron',
                'ethereum.eth'  => 'ETH - Ethereum',
                'bsc.bnb'       => 'BNB - BSC',
            ],
            'Default'      => 'usdt.trc20',
            'Description'  => '交易类型',
        ],
        'fiat'         => [
            'FriendlyName' => 'Fiat',
            'Type'         => 'dropdown',
            'Options'      => [
                'CNY' => '人民币',
                'USD' => '美元',
                'EUR' => '欧元',
                'GBP' => '英镑',
                'JPY' => '日元',
            ],
            'Default'      => 'CNY',
            'Description'  => '法币类型',
        ],
        'endpoint'     => [
            'FriendlyName' => 'Endpoint',
            'Type'         => 'text',
            'Size'         => 128,
            'Default'      => '',
            'Description'  => 'EPUSDT API endpoint',
        ],
        'secret'       => [
            'FriendlyName' => 'Token',
            'Type'         => 'text',
            'Size'         => 128,
            'Default'      => '',
            'Description'  => 'The EPUSDT token',
        ],
        'address'      => [
            'FriendlyName' => 'Address',
            'Type'         => 'text',
            'Size'         => 128,
            'Default'      => '',
            'Description'  => '收款钱包地址，留空则由 BEpusdt 自动分配',
        ],
        'timeout'      => [
            'FriendlyName' => 'Timeout',
            'Type'         => 'text',
            'Size'         => 128,
            'Default'      => '',
            'Description'  => '订单过期时间，单位秒，留空则使用默认值1200秒',
        ],
        'rate'         => [
            'FriendlyName' => 'Rate',
            'Type'         => 'text',
            'Size'         => 128,
            'Default'      => '',
            'Description'  => '浮动汇率，留空则使用 BEpusdt 配置汇率；不懂请留空',
        ],
    ];
}

function epusdt_link(array $parameters)
{
    $epusdt  = new Epusdt($parameters['endpoint'], $parameters['secret']);
    $payload = [
        'name'         => (string)$parameters['invoiceid'],
        'address'      => (string)$parameters['address'],
        'trade_type'   => (string)$parameters['trade_type'],
        'fiat'         => (string)$parameters['fiat'],
        'order_id'     => (string)$parameters['invoiceid'],
        'timeout'      => (int)$parameters['timeout'],
        'amount'       => (float)$parameters['amount'],
        'rate'         => (string)$parameters['rate'],
        'notify_url'   => rtrim($parameters['systemurl'], '/') . '/modules/gateways/callback/epusdt_notify.php',
        'redirect_url' => rtrim($parameters['systemurl'], '/') . '/modules/gateways/callback/epusdt_return.php?order_id=' . $parameters['invoiceid'],
    ];

    try {
        $redirectURL = $epusdt->redirectURL($payload);
    } catch (Exception $e) {
        return "支付接口错误: " . $e->getMessage();
    }

    return sprintf('<script type="text/javascript">window.location.href="%s";</script>', $redirectURL);
}
