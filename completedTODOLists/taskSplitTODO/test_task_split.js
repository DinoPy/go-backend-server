const WebSocket = require('ws');

function testTaskSplit(testName, connectMessage, taskSplitMessage, expectedResult = 'success') {
  return new Promise((resolve) => {
    console.log(`\n🧪 Testing: ${testName}`);
    
    const ws = new WebSocket('ws://localhost:8080/ws/taskbar');
    let connected = false;
    let taskCreated = false;
    let taskSplitResult = null;
    let createdTaskId = null;
    let splitTasksCreated = 0;
    
    ws.on('open', function open() {
      console.log('🔌 Connected to WebSocket');
      ws.send(JSON.stringify(connectMessage));
    });

    ws.on('message', function message(data) {
      try {
        const msg = JSON.parse(data.toString());
        console.log('📨 Received:', msg.event, msg.data ? (msg.data.id || msg.data.title || msg.data.type || msg.data.message) : '');
        
        if (msg.event === 'connected') {
          console.log('✅ User connected successfully');
          connected = true;
          
          // Create a test task first
          if (!taskCreated) {
            console.log('📝 Creating test task...');
            ws.send(JSON.stringify({
              event: 'task_create',
              data: {
                title: 'Test Task for Splitting',
                description: 'This task will be split into multiple parts',
                duration: '02:00:00',
                category: 'test',
                tags: ['test', 'split'],
                priority: 1,
                due_at: null,
                show_before_due_time: null
              }
            }));
          }
        } else if (msg.event === 'new_task_created' && !taskCreated) {
          console.log('✅ Test task created:', msg.data.id);
          taskCreated = true;
          createdTaskId = msg.data.id;
          
          // Now test the task split
          console.log('✂️ Testing task split...');
          const splitMessage = {
            ...taskSplitMessage,
            data: {
              ...taskSplitMessage.data,
              task_id: createdTaskId // Use the created task's ID
            }
          };
          ws.send(JSON.stringify(splitMessage));
        } else if (msg.event === 'related_task_deleted') {
          console.log('✅ Original task deleted:', msg.data.id);
        } else if (msg.event === 'new_task_created' && taskCreated) {
          splitTasksCreated++;
          console.log('✅ Split task created:', msg.data.title, `(${splitTasksCreated}/2)`);
          if (splitTasksCreated === 2) {
            taskSplitResult = 'success';
          }
        } else if (msg.event === 'connection_error') {
          console.log('❌ Error:', msg.data.type, '-', msg.data.message);
          taskSplitResult = 'error';
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
      taskSplitResult = 'error';
    });

    ws.on('close', function close(code, reason) {
      console.log('🔌 Connection closed:', code, reason.toString());
      if (expectedResult === 'success' && taskSplitResult === 'success') {
        console.log('✅ Test PASSED');
      } else if (expectedResult === 'error' && taskSplitResult === 'error') {
        console.log('✅ Test PASSED (expected error)');
      } else {
        console.log('❌ Test FAILED - Expected:', expectedResult, 'Got:', taskSplitResult);
      }
      resolve();
    });

    // Close after 8 seconds
    setTimeout(() => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    }, 8000);
  });
}

async function runTaskSplitTests() {
  console.log('🚀 Starting Task Split Tests...\n');
  
  // Test 1: Valid task split with 2 parts
  await testTaskSplit('Valid task split with 2 parts', {
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
      task_id: 'placeholder', // Will be replaced with actual task ID
      splits: [
        {
          title: 'Part 1',
          description: 'First part description',
          duration: '01:30:00'
        },
        {
          title: 'Part 2',
          description: 'Second part description',
          duration: '00:30:00'
        }
      ]
    }
  });

  // Wait a bit between tests
  await new Promise(resolve => setTimeout(resolve, 3000));

  // Test 2: Empty splits array (should fail)
  await testTaskSplit('Empty splits array (should fail)', {
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
      task_id: 'placeholder',
      splits: []
    }
  }, 'error');

  // Wait a bit between tests
  await new Promise(resolve => setTimeout(resolve, 3000));

  // Test 3: Invalid task ID (should fail)
  await testTaskSplit('Invalid task ID (should fail)', {
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
}

runTaskSplitTests().then(() => {
  console.log('\n🏁 All task split tests completed');
  process.exit(0);
});