import blessed from 'blessed';
import dotenv from 'dotenv';
import { createClient } from '@supabase/supabase-js';
import bcrypt from 'bcrypt';

dotenv.config();

const supabaseUrl = process.env.SUPABASE_URL as string;
const supabaseAnonKey = process.env.SUPABASE_ANON_KEY as string;

const supabase = createClient(supabaseUrl, supabaseAnonKey);

interface Message {
  id: number;
  created_at: string;
  username: string;
  content: string;
}

interface User {
  id: number;
  created_at: string;
  username: string;
  password_hash: string;
}

async function registerUser(username: string, password_hash: string): Promise<void> {
  const { error } = await supabase.from('users').insert([{ username, password_hash }]);
  if (error) {
    throw error;
  }
}

async function loginUser(username: string, password_hash: string): Promise<User | null> {
  const { data, error } = await supabase.from('users').select('*').eq('username', username).single();
  if (error) {
    console.error('Login error:', error);
    return null;
  }
  if (data && await bcrypt.compare(password_hash, data.password_hash)) {
    return data as User;
  }
  return null;
}

async function getMessages(): Promise<Message[]> {
  const { data, error } = await supabase.from('messages').select('*').order('created_at', { ascending: true });
  if (error) {
    throw error;
  }
  return data as Message[];
}

async function sendMessage(username: string, content: string): Promise<void> {
  const { error } = await supabase.from('messages').insert([{ username, content }]);
  if (error) {
    throw error;
  }
}

async function setupDatabase(): Promise<void> {
  // For simplicity, we are not creating tables via migration in this example.
  // In a real application, you would use Supabase migrations or the Supabase UI to set up your tables.
  // Ensure you have 'users' and 'messages' tables created in your Supabase project.
}

async function main(): Promise<void> {
  await setupDatabase();

  const screen = blessed.screen({
    smartCSR: true,
    title: 'Chat App',
  });

  const chatBox = blessed.box({
    top: 0,
    left: 0,
    width: '100%',
    height: '100%-1',
    content: '',
    scrollable: true,
    alwaysScroll: true,
    tags: true,
    border: {
      type: 'line',
    },
    style: {
      fg: 'white',
      bg: 'black',
      border: {
        fg: '#f0f0f0',
      },
    },
  });

  const input = blessed.textbox({
    bottom: 0,
    left: 0,
    width: '100%',
    height: 1,
    inputOnFocus: true,
    padding: {
      left: 1,
      right: 1,
    },
    style: {
      fg: 'white',
      bg: 'blue',
    },
  });

  screen.append(chatBox);
  screen.append(input);

  screen.key(['escape', 'q', 'C-c'], () => process.exit(0));

  input.key('enter', async () => {
    const content = input.getValue();
    input.clearValue();
    screen.render();

    if (content.startsWith('/register ')) {
      const parts = content.split(' ');
      if (parts.length === 3) {
        const username = parts[1];
        const password = parts[2];
        const password_hash = await bcrypt.hash(password, 10);
        try {
          await registerUser(username, password_hash);
          chatBox.insertBottom(`{green-fg}User ${username} registered successfully!{/green-fg}`);
        } catch (e: any) {
          chatBox.insertBottom(`{red-fg}Registration failed: ${e.message}{/red-fg}`);
        }
      } else {
        chatBox.insertBottom('{yellow-fg}Usage: /register <username> <password>{/yellow-fg}');
      }
    } else if (content.startsWith('/login ')) {
      const parts = content.split(' ');
      if (parts.length === 3) {
        const username = parts[1];
        const password = parts[2];
        const user = await loginUser(username, password);
        if (user) {
          chatBox.insertBottom(`{green-fg}User ${username} logged in successfully!{/green-fg}`);
        } else {
          chatBox.insertBottom('{red-fg}Login failed: Invalid username or password.{/red-fg}');
        }
      } else {
        chatBox.insertBottom('{yellow-fg}Usage: /login <username> <password>{/yellow-fg}');
      }
    } else if (content.trim()) {
      await sendMessage('anonymous', content);
    }
    screen.render();
  });

  screen.render();

  // Initial message load and real-time updates
  const loadMessages = async () => {
    const messages = await getMessages();
    chatBox.setContent('');
    messages.forEach(msg => {
      chatBox.insertBottom(`{bold}${msg.username}{/bold}: ${msg.content}`);
    });
    screen.render();
  };

  await loadMessages();

  supabase.from('messages').on('*', () => {
    loadMessages();
  }).subscribe();

  input.focus();
}

main().catch(console.error);