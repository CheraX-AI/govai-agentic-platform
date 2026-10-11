import app from './app.js';
import config from './config/index.js';

app.listen(config.port, () => {
  console.log(`GovAI backend running on http://localhost:${config.port} (${config.env})`);
});
