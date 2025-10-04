const WebSocket = require('ws');

function testEdgeCase(testName, connectMessage, taskSplitMessage, expectedResult = 'error') {
  return new Promise((resolve) => {
    console.log(`\n🧪 Testing: ${testName}`);
    
    const ws = new WebSocket('ws://localhost:8080/ws/taskbar');
    let connected = false;
    let result = null;
    
    ws.on('open', function open() {
      console.log('🔌 Connected to WebSocket');
      ws.send(JSON.stringify(connectMessage));
    });

    ws.on('message', function message(data) {
      try {
        const msg = JSON.parse(data.toString());
        console.log('📨 Received:', msg.event, msg.data ? (msg.data.type || msg.data.message || msg.data.id) : '');
        
        if (msg.event === 'connected') {
          console.log('✅ User connected successfully');
          connected = true;
          
          // Send the task split message
          console.log('✂️ Testing task split...');
          ws.send(JSON.stringify(taskSplitMessage));
        } else if (msg.event === 'connection_error') {
          console.log('❌ Error:', msg.data.type, '-', msg.data.message);
          result = 'error';
        } else if (msg.event === 'related_task_deleted') {
          console.log('✅ Original task deleted:', msg.data.id);
          result = 'success';
        } else if (msg.event === 'new_task_created') {
          console.log('✅ Split task created:', msg.data.title);
          result = 'success';
        } else if (msg.event === 'ping') {
          // Ignore ping messages
        } else {
          console.log('📨 Other event:', msg.event);
        }
      } catch (e) {
        console.log('📨 Raw message:', data.toString());
      }
    });

    ws.on('error', function error(err) {
      console.error('❌ WebSocket error:', err.message);
      result = 'error';
    });

    ws.on('close', function close(code, reason) {
      console.log('🔌 Connection closed:', code, reason.toString());
      if (expectedResult === 'success' && result === 'success') {
        console.log('✅ Test PASSED');
      } else if (expectedResult === 'error' && result === 'error') {
        console.log('✅ Test PASSED (expected error)');
      } else {
        console.log('❌ Test FAILED - Expected:', expectedResult, 'Got:', result);
      }
      resolve();
    });

    // Close after 5 seconds
    setTimeout(() => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    }, 5000);
  });
}

async function runEdgeCaseTests() {
  console.log('🚀 Starting Edge Case Tests...\n');
  
  // Test 1: Empty splits array (already tested, but let's confirm)
  await testEdgeCase('Empty splits array', {
    event: 'connect',
    data: {
      id: '28c07fc5-2732-47c0-b305-92982fbddcef',
      email: 'test@example.com',
      first_name: 'Test',
      last_name: 'User',
      google_uid: '1234567890abcdef'
    }
  }, {
    event: 'task_split',
    data: {
      task_id: '00000000-0000-0000-0000-000000000000',
      splits: []
    }
  }, 'error');

  await new Promise(resolve => setTimeout(resolve, 2000));

  // Test 2: Non-existent task ID
  await testEdgeCase('Non-existent task ID', {
    event: 'connect',
    data: {
      id: '28c07fc5-2732-47c0-b305-92982fbddcef',
      email: 'test@example.com',
      first_name: 'Test',
      last_name: 'User',
      google_uid: '1234567890abcdef'
    }
  }, {
    event: 'task_split',
    data: {
      task_id: '00000000-0000-0000-0000-000000000000',
      splits: [
        {
          title: 'Test Split',
          description: 'Test description',
          duration: '01:00:00'
        }
      ]
    }
  }, 'error');

  await new Promise(resolve => setTimeout(resolve, 2000));

  // Test 3: Unauthorized access (different user trying to split another user's task)
  // We'll use a task ID that might exist but belongs to a different user
  await testEdgeCase('Unauthorized access (different user)', {
    event: 'connect',
    data: {
      id: '11111111-1111-1111-1111-111111111111', // Different user ID
      email: 'other@example.com',
      first_name: 'Other',
      last_name: 'User',
      google_uid: 'different_google_uid'
    }
  }, {
    event: 'task_split',
    data: {
      task_id: '00000000-0000-0000-0000-000000000000', // This will be not_found, but if it existed, it would be unauthorized
      splits: [
        {
          title: 'Unauthorized Split',
          description: 'This should fail',
          duration: '01:00:00'
        }
      ]
    }
  }, 'error');

  await new Promise(resolve => setTimeout(resolve, 2000));

  // Test 4: Invalid JSON structure
  await testEdgeCase('Invalid JSON structure', {
    event: 'connect',
    data: {
      id: '28c07fc5-2732-47c0-b305-92982fbddcef',
      email: 'test@example.com',
      first_name: 'Test',
      last_name: 'User',
      google_uid: '1234567890abcdef'
    }
  }, {
    event: 'task_split',
    data: {
      // Missing splits field
      task_id: '00000000-0000-0000-0000-000000000000'
    }
  }, 'error');

  await new Promise(resolve => setTimeout(resolve, 2000));

  // Test 5: Valid UUID format but non-existent task
  await testEdgeCase('Valid UUID format but non-existent task', {
    event: 'connect',
    data: {
      id: '28c07fc5-2732-47c0-b305-92982fbddcef',
      email: 'test@example.com',
      first_name: 'Test',
      last_name: 'User',
      google_uid: '1234567890abcdef'
    }
  }, {
    event: 'task_split',
    data: {
      task_id: '11111111-1111-1111-1111-111111111111', // Valid UUID format but non-existent
      splits: [
        {
          title: 'Test Split',
          description: 'Test description',
          duration: '01:00:00'
        }
      ]
    }
  }, 'error');
}

runEdgeCaseTests().then(() => {
  console.log('\n🏁 All edge case tests completed');
  process.exit(0);
});
