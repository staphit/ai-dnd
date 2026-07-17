import React from 'react';
import ReactDOM from 'react-dom/client';
import '@fontsource/cinzel/400.css';
import '@fontsource/cinzel/500.css';
import '@fontsource/cinzel/600.css';
import '@fontsource-variable/source-serif-4';
import '@fontsource-variable/jetbrains-mono';
import '../../tokens.css';
import './styles.css';
import './feature-vars.css';
import './features.css';
import App from './App';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode><App /></React.StrictMode>,
);
