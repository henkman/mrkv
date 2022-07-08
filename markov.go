package mrkv

import (
	"bufio"
	"database/sql"
	"errors"
	"io"
	"math/rand"
	"strings"
	"unicode"

	_ "github.com/mattn/go-sqlite3"
)

type ID uint64

type Word struct {
	Word string
	Next []ID
}

type Markov struct {
	rand  *rand.Rand
	words map[ID]Word
}

func (m *Markov) Init(seed int64) {
	m.rand = rand.New(rand.NewSource(seed))
	m.words = map[ID]Word{}
}

func (m *Markov) InitFromDB(filepath string, seed int64) error {
	m.rand = rand.New(rand.NewSource(seed))
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return err
	}
	wordRows, err := db.Query("SELECT id, word FROM word")
	if err != nil {
		return err
	}
	m.words = map[ID]Word{}
	for wordRows.Next() {
		var id ID
		var word string
		err = wordRows.Scan(&id, &word)
		if err != nil {
			return err
		}
		m.words[id] = Word{Word: word}
	}
	nextRows, err := db.Query("SELECT id, next FROM next")
	if err != nil {
		return err
	}
	for nextRows.Next() {
		var id, nid ID
		err = nextRows.Scan(&id, &nid)
		if err != nil {
			return err
		}
		word := m.words[id]
		word.Next = append(word.Next, nid)
		m.words[id] = word
	}
	return nil
}

func (m *Markov) SaveToDB(filepath string) error {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return err
	}

	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS word (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    word VARCHAR UNIQUE
);
DELETE FROM word;
CREATE TABLE IF NOT EXISTS next (
    id   BIGINT,
    next BIGINT,
    PRIMARY KEY (
        id,
        next
    ),
    UNIQUE (
        id,
        next
    )
);
DELETE FROM next;`); err != nil {
		return err
	}
	{
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		wordInsert, err := tx.Prepare("INSERT INTO word(id, word) VALUES(?, ?)")
		if err != nil {
			return err
		}
		for id, word := range m.words {
			_, err = wordInsert.Exec(id, word.Word)
			if err != nil {
				return err
			}
		}
		err = tx.Commit()
		if err != nil {
			return err
		}
	}
	{
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		nextInsert, err := tx.Prepare("INSERT INTO next(id, next) VALUES(?, ?)")
		if err != nil {
			return err
		}
		for id, word := range m.words {
			for _, next := range word.Next {
				_, err = nextInsert.Exec(id, next)
				if err != nil {
					return err
				}
			}
		}
		err = tx.Commit()
		if err != nil {
			return err
		}
	}
	return nil
}

var errorWordNotFound = errors.New("word not found")

func (m *Markov) findWordID(word string) (ID, error) {
	for id, w := range m.words {
		if word == w.Word {
			return id, nil
		}
	}
	return 0, errorWordNotFound
}

func (m *Markov) addNext(word, next string) {
	nid, err := m.findWordID(next)
	if err != nil {
		nid = ID(len(m.words))
		m.words[nid] = Word{Word: next}
	}

	wid, err := m.findWordID(word)
	if err != nil {
		wid = ID(len(m.words))
		m.words[wid] = Word{Word: word, Next: []ID{nid}}
	} else {
		w := m.words[wid]
		for _, id := range w.Next {
			if id == nid {
				return
			}
		}
		w.Next = append(w.Next, nid)
		m.words[wid] = w
	}
}

func (m *Markov) Feed(in io.Reader) {
	type MatchFun func(rune) bool
	br := bufio.NewReader(in)
	read := func() rune {
		r, _, err := br.ReadRune()
		if err != nil {
			return -1
		}
		return r
	}
	accept := func(r rune) MatchFun {
		for _, mf := range []MatchFun{
			unicode.IsLetter,
			unicode.IsDigit,
			unicode.IsPunct,
		} {
			if mf(r) {
				return mf
			}
		}
		return nil
	}
	var sb strings.Builder
	last := ""
	c := read()
next:
	for c != -1 {
		mf := accept(c)
		for mf == nil {
			c = read()
			if c == -1 {
				break next
			}
			mf = accept(c)
		}
		sb.Reset()
		sb.WriteRune(c)
		for {
			c = read()
			if c == -1 || !mf(c) {
				break
			}
			sb.WriteRune(c)
		}
		if last != "" {
			m.addNext(last, sb.String())
		}
		last = sb.String()
	}
}

func (m *Markov) Generate(chainLength uint) []string {
	n := m.rand.Int63n(int64(len(m.words)))
	word := m.words[ID(n)]
	chain := make([]string, 0, chainLength)
	chain = append(chain, word.Word)
	var i uint
	for i = 1; i < chainLength; i++ {
		if len(word.Next) == 0 {
			break
		}
		var next Word
		if len(word.Next) == 1 {
			next = m.words[word.Next[0]]
		} else {
			n := m.rand.Int63n(int64(len(word.Next)))
			next = m.words[word.Next[n]]
		}
		chain = append(chain, next.Word)
		word = next
	}
	return chain
}

func WordJoin(words []string) string {
	text := ""
	for i, _ := range words {
		text += words[i]
		isLast := i == (len(words) - 1)
		if !isLast {
			next := words[i+1]
			fc := []rune(next)[0]
			word := []rune(words[i])
			lc := word[len(word)-1]
			if lc == '.' || lc == ',' || lc == '?' ||
				lc == '!' || lc == ';' || lc == ':' ||
				(unicode.IsLetter(lc) || unicode.IsDigit(lc)) &&
					(unicode.IsLetter(fc) || unicode.IsDigit(fc)) {
				text += " "
			}
		}
	}
	return text
}
