#!/usr/bin/env node
// Generate ?q=base64url sandbox links for docs.
// Usage: node scripts/sandbox-links.js [query...]
function b64url(s) {
  return Buffer.from(s, 'utf8').toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}
const BASE = 'https://sandbox.gdql.dev';
const args = process.argv.slice(2);
if (args.length === 0) {
  const queries = [
    'SHOWS;',
    'SHOWS FROM 1977;',
    'SHOWS FROM 1977-1980;',
    'SHOWS FROM PRIMAL;',
    'SHOWS FROM 1977 WHERE "Scarlet Begonias" > "Fire on the Mountain";',
    'SHOWS FROM 77-80 LIMIT 10;',
    'SHOWS FROM 1977 ORDER BY DATE DESC;',
    'SHOWS FROM 1977 AS JSON;',
    'SONGS;',
    'SONGS WITH LYRICS("train", "road");',
    'SONGS WRITTEN 1968-1970;',
    'SONGS WITH LYRICS("rose") WRITTEN 1970 LIMIT 20;',
    'PERFORMANCES OF "Dark Star";',
    'PERFORMANCES OF "Dark Star" FROM 1972-1974;',
    'PERFORMANCES OF "Dark Star" WITH LENGTH > 20min;',
    'PERFORMANCES OF "Eyes of the World" ORDER BY LENGTH DESC LIMIT 5;',
    'SETLIST FOR 5/8/77;',
    'SETLIST FOR "Cornell 1977";',
    'SHOWS FROM 77-80 WHERE "Scarlet Begonias" > "Fire on the Mountain";',
    'SONGS WITH LYRICS("train", "road", "rose") WRITTEN 1968-1970;',
    'SHOWS WHERE "Help on the Way" > "Slipknot!" > "Franklin\'s Tower";',
    'PERFORMANCES OF "Dark Star" FROM 1972 WITH LENGTH > 20min;',
    'SHOWS FROM 77-79 WHERE "Scarlet Begonias" > "Fire on the Mountain";',
    'SONGS WITH LYRICS("train", "railroad", "engineer");',
    'PERFORMANCES OF "Dark Star" ORDER BY LENGTH DESC LIMIT 10;',
    'SHOWS FROM 1969 WHERE PLAYED "St. Stephen";',
    'SHOWS FROM 1969 WHERE PLAYED "St. Stephen" > "The Eleven";',
  ];
  queries.forEach((q) => console.log(`${BASE}?q=${b64url(q)}&run=1`));
} else {
  args.forEach((q) => console.log(`${BASE}?q=${b64url(q)}&run=1`));
}
