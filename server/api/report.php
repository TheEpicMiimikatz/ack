<?php

if ($_SERVER['REQUEST_METHOD'] === 'GET') {
    if (!isset($_COOKIE['_auth_id'])) {
        header('Content-Type: application/json', true, 401);
        echo json_encode(['status' => 'error', 'reason' => 'unauthorized']);
        exit;
    }

    $pid = $_GET['proc_id'];
    $task_id = $_GET['task_id'];
    $status = $_GET['status'];

    file_put_contents('bots.log', $_SERVER['REMOTE_ADDR'] . " status: $status task_id: $task_id process id: $pid\n");
}
