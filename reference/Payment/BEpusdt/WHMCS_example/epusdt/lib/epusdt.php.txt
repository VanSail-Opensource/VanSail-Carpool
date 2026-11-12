<?php

namespace gateways\epusdt\lib;

use Exception;

class Epusdt
{
    /**
     * The epusdt API endpoint.
     *
     * @var string
     */
    protected string $endpoint;

    /**
     * The epusdt token.
     *
     * @var string
     */
    protected string $token;

    /**
     * Create epusdt instance.
     *
     * @param string $endpoint
     * @param string $token
     *
     */
    public function __construct(string $endpoint, string $token)
    {
        $this->endpoint = rtrim($endpoint, '/');
        $this->token    = $token;
    }

    public function curl_post($url, $para = '')
    {

        $header[] = 'Accept: */*';
        $header[] = 'Accept-Language: zh-CN,zh;q=0.8';
        $header[] = 'Connection: close';
        $header[] = 'Content-Type: application/json';

        $ch = curl_init($url);
        curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, true);
        curl_setopt($ch, CURLOPT_SSL_VERIFYHOST, true);
        curl_setopt($ch, CURLOPT_HTTPHEADER, $header);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($ch, CURLOPT_POST, true);
        curl_setopt($ch, CURLOPT_POSTFIELDS, $para);
        curl_setopt($ch, CURLOPT_TIMEOUT, 10);
        curl_setopt($ch, CURLOPT_HEADER, false);
        $res = curl_exec($ch);
        $err = curl_error($ch);
        if ($err) {
            throw new Exception($err);
        }

        curl_close($ch);

        return $res;
    }

    protected function signature(array $parameter): string
    {
        ksort($parameter);

        $sign = '';

        foreach ($parameter as $key => $val) {
            if ($val == '') continue;
            if ($key != 'signature') {
                if ($sign != '') {
                    $sign .= "&";

                }

                $sign .= "$key=$val";
            }
        }

        return md5($sign . $this->token);
    }

    /**
     * Verify the signature.
     *
     * @param string $signature
     * @param array $parameters
     *
     * @return  bool
     */
    public function verifySignature(string $signature, array $parameters = []): bool
    {
        return $signature === $this->signature($parameters);
    }

    /**
     * Generate the payment link.
     *
     * @param array $parameters
     *
     * @return  string
     * @throws Exception
     */
    public function redirectURL(array $parameters = [])
    {
        try {
            $parameters['signature'] = $this->signature($parameters);

            $res = $this->curl_post(
                $this->endpoint . '/api/v1/order/create-transaction',
                json_encode(
                    $parameters,
                    JSON_UNESCAPED_SLASHES
                )
            );

            $res = json_decode($res, true);

            if ($res["status_code"] != 200) {
                throw new Exception($res["message"]);
            }

            return $res['data']['payment_url'];
        } catch (Exception $e) {
            throw $e;
        }
    }
}
