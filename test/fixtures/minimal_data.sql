-- Minimal test data: Cornell 5/8/77 + a couple other shows, Scarlet > Fire

INSERT INTO venues (id, name, city, state, country) VALUES
(1, 'Barton Hall', 'Ithaca', 'NY', 'USA'),
(2, 'Winterland Arena', 'San Francisco', 'CA', 'USA'),
(3, 'Capital Centre', 'Landover', 'MD', 'USA');

INSERT INTO shows (id, date, venue_id, tour, notes, rating) VALUES
(1, '1977-05-08', 1, 'Spring 1977', 'Cornell 77', 4.9),
(2, '1977-02-26', 2, 'Winter 1977', NULL, 4.5),
(3, '1978-04-24', 3, 'Spring 1978', NULL, 4.2);

INSERT INTO songs (id, name, short_name, writers, first_played, last_played, times_played) VALUES
(1, 'Scarlet Begonias', 'Scarlet', 'Hunter/Garcia', '1974-03-23', '1995-07-09', 314),
(2, 'Fire on the Mountain', 'Fire', 'Hunter/Hart', '1977-03-18', '1995-07-09', 303),
(3, 'Help on the Way', 'Help', 'Hunter/Garcia', '1975-08-13', '1995-07-09', 306),
(4, 'Samson and Delilah', 'Samson', 'traditional', '1976-06-03', '1995-06-25', 287),
(5, 'Morning Dew', 'Dew', 'Dobson/Rose', '1967-03-18', '1995-06-25', 232);

-- Cornell 77 set 2: Scarlet > Fire and more (segue_type = transition TO next, so on Scarlet row)
INSERT INTO performances (id, show_id, song_id, set_number, position, segue_type, length_seconds, is_opener, is_closer) VALUES
(1, 1, 1, 2, 1, '>', 580, 1, 0),
(2, 1, 2, 2, 2, NULL, 620, 0, 0),
(3, 1, 3, 2, 3, '>>', 320, 0, 0),
(4, 1, 4, 2, 4, '>', 420, 0, 0),
(5, 1, 5, 2, 5, NULL, 720, 0, 1);

-- Winterland 2/26/77: Scarlet > Fire
INSERT INTO performances (id, show_id, song_id, set_number, position, segue_type, length_seconds, is_opener, is_closer) VALUES
(6, 2, 1, 2, 3, '>', 540, 0, 0),
(7, 2, 2, 2, 4, NULL, 600, 0, 0);

-- Landover 78: SET2 OPENED with Samson
INSERT INTO performances (id, show_id, song_id, set_number, position, segue_type, length_seconds, is_opener, is_closer) VALUES
(8, 3, 4, 2, 1, NULL, 400, 1, 0),
(9, 3, 1, 2, 2, '>', 560, 0, 0),
(10, 3, 2, 2, 3, NULL, 610, 0, 0);

-- Lyrics for SONGS WITH LYRICS test
INSERT INTO lyrics (song_id, lyrics, lyrics_fts) VALUES
(1, 'As I was walkin round the town...', 'As I was walkin round the town'),
(2, 'Long distance runner...', 'Long distance runner'),
(4, 'If I had my way I would tear this old building down', 'If I had my way I would tear this old building down');
