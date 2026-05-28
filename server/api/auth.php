<?php
session_start();

if ($_SERVER['REQUEST_METHOD'] === 'GET') {
    $token=$_SERVER['HTTP_X_API_TOKEN'];

    if ($token != '<CUSTOM-API-TOKEN-HERE>') {
        header('Content-Type: application/json', true, 401);
        echo json_encode(['status' => 'error', 'reason' => 'unauthorized']);
        exit;
    }

    $bot_id = uniqid();

    setcookie('.auth_id', $bot_id);
}
