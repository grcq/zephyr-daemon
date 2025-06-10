const express = require('express');
const app = express();

app.get('/', (req, res) => {
    res.send('Hello, World!');
});

app.listen(25565, () => {
    console.log('Server is running on port 25565');
});