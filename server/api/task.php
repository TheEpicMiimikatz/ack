<?php

if ($_SERVER['REQUEST_METHOD'] === 'GET') {
    $task = [
        'task_id' => uniqid(),
        'task_status' => 'ready',
        'package_name' => '<YOUR-PACKAGE-ZIP-HERE>.zip',
        'hide_window' => true
    ];

    $package_path = $_SERVER['DOCUMENT_ROOT'] . "/packages/" . $task['package_name'];

    $task['package_hash'] = hash('sha256', file_get_contents($package_path));

    header('Content-Type: application/json', $true, 200);
    echo json_encode($task);
}

