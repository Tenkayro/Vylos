import './styles/app.css';
import { mount } from 'svelte';
import App from './App.svelte';

const target = document.getElementById('app')!;
mount(App, { target });
