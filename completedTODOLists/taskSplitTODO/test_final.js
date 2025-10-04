const WebSocket = require('ws');

function testConnection(testName, connectMessage, expectedError = null) {
  return new Promise((resolve) => {
    console.log(`\n🧪 Testing: ${testName}`);
    
    const ws = new WebSocket('ws://localhost:8080/ws/taskbar');
    
    ws.on('open', function open() {
      ws.send(JSON.stringify(connectMessage));
    });

    ws.on('message', function message(data) {
      try {
        const msg = JSON.parse(data.toString());
        if (msg.event === 'connected') {
          console.log('✅ Success: User connected');
          if (expectedError) {
            console.log('❌ Expected error but got success');
          }
        } else if (msg.event === 'connection_error') {
          console.log('❌ Error:', msg.data.type, '-', msg.data.message);
          if (expectedError && msg.data.type === expectedError) {
            console.log('✅ Expected error received');
          } else if (expectedError) {
            console.log('❌ Expected', expectedError, 'but got', msg.data.type);
          }
        } else {
          console.log('📨 Other:', msg.event);
        }
      } catch (e) {
        console.log('📨 Raw:', data.toString());
      }
    });

    ws.on('error', function error(err) {
      console.error('❌ WebSocket error:', err.message);
    });

    ws.on('close', function close() {
      console.log('🔌 Connection closed');
      resolve();
    });

    // Close after 2 seconds
    setTimeout(() => {
      ws.close();
    }, 2000);
  });
}

async function runTests() {
  // Test 1: Connection without Google UID
  await testConnection('Connection without Google UID', {
    event: 'connect',
    data: {
      id: '28c07fc5-2732-47c0-b305-92982fbddcef',
      email: 'test2@example.com',
      first_name: 'Test2',
      last_name: 'User2'
      // Missing google_uid
    }
  }, 'invalid_google_uid');

  // Test 2: Connection with mismatched Google UID
  await testConnection('Connection with mismatched Google UID', {
    event: 'connect',
    data: {
      id: '28c07fc5-2732-47c0-b305-92982fbddcef',
      email: 'test@example.com', // Same email as existing user
      first_name: 'Test',
      last_name: 'User',
      google_uid: 'different_google_uid' // Different from existing
    }
  }, 'google_uid_mismatch');

  // Test 3: Connection with existing user and correct Google UID
  await testConnection('Connection with existing user and correct Google UID', {
    event: 'connect',
    data: {
      id: '28c07fc5-2732-47c0-b305-92982fbddcef',
      email: 'test@example.com', // Same email as existing user
      first_name: 'Test',
      last_name: 'User',
      google_uid: '1234567890abcdef' // Same as existing
    }
  }); // Should succeed

  // Test 4: New user with valid Google UID
  await testConnection('New user with valid Google UID', {
    event: 'connect',
    data: {
      id: '28c07fc5-2732-47c0-b305-92982fbddcef',
      email: 'newuser@example.com',
      first_name: 'New',
      last_name: 'User',
      google_uid: 'new_google_uid_123'
    }
  }); // Should succeed
}

runTests().then(() => {
  console.log('\n🏁 All tests completed');
  process.exit(0);
});
