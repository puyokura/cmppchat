const blessed = require('blessed');
const { createClient } = require('@supabase/supabase-js');

// Supabase configuration (replace with your actual keys and URL)
// IMPORTANT: In a real application, these should be loaded securely (e.g., environment variables).
const supabaseUrl = 'https://lpfxspzgmmibonkesiga.supabase.co'; // Replace with your Supabase URL
const supabaseKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImxwZnhzcHpnbW1pYm9ua2VzaWdhIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTA1MTI1NzgsImV4cCI6MjA2NjA4ODU3OH0.mfR9ahzSMKAPn5f77Ldn9VkJTBkkD_ujZQgcy7IfwwU'; // Replace with your Supabase anon key
const supabase = createClient(supabaseUrl, supabaseKey);

// Create a screen object.
const screen = blessed.screen({
  smartCSR: true
});

let currentUser = null;

screen.title = 'cppchat';

// Create a box for messages.
const messageBox = blessed.log({
  parent: screen,
  top: 0,
  left: 0,
  width: '100%',
  height: 'shrink',
  tags: true,
  scrollable: true,
  alwaysScroll: true,
  scrollbar: {},
  keys: true,
  vi: true,
  mouse: true,
  border: 'line',
  style: {
    fg: 'default',
    bg: 'default',
    border: { fg: 'default' }
  }
});

// Create a box for input.
const input = blessed.textbox({
  parent: screen,
  bottom: 0,
  left: 0,
  width: '100%',
  height: 3,
  tags: true,
  inputOnFocus: true,
  border: 'line',
  style: {
    fg: 'default',
    bg: 'default',
    border: { fg: 'default' }
  }
});

// Append our boxes to the screen.
screen.append(messageBox);
screen.append(input);

// Add a keypress handler for the escape key to exit.
screen.key(['escape', 'q', 'C-c'], function(ch, key) {
  return process.exit(0);
});

// Focus the input box.
input.focus();

// Render the screen.
screen.render();

// Handle input
input.on('submit', async function(text) {
  const command = text.trim();
  input.clearValue();
  screen.render();

  if (command === '') {
    return;
  }

  // Basic command parsing
  const parts = command.split(' ');
  const cmd = parts[0];

  if (cmd === '/register') {
    // Usage: /register <username> <password>
    if (parts.length !== 3) {
      messageBox.add('{red}Usage: /register <username> <password>{/red}');
    } else {
      const userName = parts[1];
      const password = parts[2];

      // TODO: Validate username (e.g., uniqueness, allowed characters)
      // For now, generate a dummy email from the username.
      // WARNING: This approach requires disabling email confirmation in Supabase auth settings
      // and might have issues with username uniqueness if not handled carefully.
      const dummyEmail = `${userName.toLowerCase().replace(/[^a-z0-9]/g, '')}@example.com`; // Simple sanitization

      messageBox.add('Registering user...');
      try {
        // Note: Email confirmation might need to be disabled in Supabase settings for this to work without user interaction.
        const { data, error } = await supabase.auth.signUp({ email: dummyEmail, password });
        if (error) {
          messageBox.add('{red}Registration failed: ' + error.message + '{/red}');
        } else if (data && data.user) {
          messageBox.add('{green}Registration successful! User: ' + userName + '{/green}');
          if (data.session) {
             currentUser = {
               id: data.user.id,
               username: userName // Use the provided userName for display
             };
             messageBox.add('{yellow}Automatically logged in as ' + currentUser.username + '.{/yellow}');
             // Session is available if email confirmation is turned off.
             const { data: profileData, error: profileError } = await supabase
               .from('profiles')
               .insert([{ id: data.user.id, username: userName }]);
             if (profileError) {
               messageBox.add('{red}Error storing profile: ' + profileError.message + '{/red}');
             }
          } else {
             messageBox.add('{yellow}Please check your email to confirm your account.{/yellow}');
          }
        } else {
           messageBox.add('{yellow}Registration initiated. Check email for confirmation.{/yellow}');
        }
      } catch (e) {
        messageBox.add('{red}An unexpected error occurred during registration: ' + e.message + '{/red}');
      }
    }
  } else if (cmd === '/login') {
    // Usage: /login <username> <password>
    if (parts.length !== 3) {
      messageBox.add('{red}Usage: /login <username> <password>{/red}');
    } else {
      const userName = parts[1];
      const password = parts[2];

      // Generate the expected dummy email from the username used during registration.
      const dummyEmail = `${userName.toLowerCase().replace(/[^a-z0-9]/g, '')}@example.com`; // Must match registration logic

      messageBox.add('Logging in user...');
      try {
        const { data, error } = await supabase.auth.signInWithPassword({ email: dummyEmail, password });
        if (error) {
          messageBox.add('{red}Login failed: ' + error.message + '{/red}');
        } else if (data && data.user) {
          currentUser = {
            id: data.user.id,
            username: userName // Use the provided userName for display
          };
          messageBox.add('{green}Login successful! User: ' + currentUser.username + '{/green}');
        } else {
          messageBox.add('{red}Login failed: No user data received.{/red}');
        }
      } catch (e) {
        messageBox.add('{red}An unexpected error occurred during login: ' + e.message + '{/red}');
      }
    }
  } else {
    // Handle other commands or send as message later
    // For now, assume it's a message to send if not a recognized command
    // TODO: Add checks for logged-in user before sending
    await sendMessage(command);
  }

  screen.render();
});

// --- Supabase Integration ---
// Function to send a message
async function sendMessage(text) {
  const userId = currentUser ? currentUser.id : null; // Get logged-in user ID
  const userName = currentUser ? currentUser.username : 'Anonymous';

  try {
    // Assuming a 'messages' table with columns: id, user_id, user_name, content, created_at
    const { data, error } = await supabase
      .from('messages')
      .insert([{ user_id: userId, user_name: userName, content: text }]);

    if (error) {
      messageBox.add('{red}Error sending message: ' + error.message + '{/red}');
    } else {
      // Message sent successfully (Realtime will handle display)
      // Optionally, clear input here again if needed, though handled on submit
    }
  } catch (e) {
    messageBox.add('{red}An unexpected error occurred while sending message: ' + e.message + '{/red}');
  }
}

// Listen for new messages in real-time
// Assuming a 'messages' table
supabase
  .channel('messages')
  .on('postgres_changes', {
    event: 'INSERT',
    schema: 'public',
    table: 'messages',
  }, async (payload) => {
    // TODO: Format message display according to requirements (IP-like ID, username, color, wrapping)
    const { new: newMessage } = payload;
    let displayUserName = newMessage.user_name || 'Anonymous';

    // If user_name is null (e.g., from an anonymous message or old data),
    // try to fetch it from the profiles table if user_id is available.
    if (!newMessage.user_name && newMessage.user_id) {
      const { data: profile, error: profileError } = await supabase
        .from('profiles')
        .select('username')
        .eq('id', newMessage.user_id)
        .single();

      if (profileError) {
        console.error('Error fetching profile for message:', profileError);
      } else if (profile) {
        displayUserName = profile.username;
      }
    }

    const formattedMessage = `(${displayUserName}): ${newMessage.content}`;
    messageBox.add(formattedMessage);
    screen.render();
  })
  .subscribe();

// --- Initial Setup ---
messageBox.add('{bold}Welcome to cppchat!{/bold}');
messageBox.add('{yellow}Please use /register <username> <password> or /login <username> <password> to get started.{/yellow}');
messageBox.add(''); // Add an empty line for spacing
screen.render();