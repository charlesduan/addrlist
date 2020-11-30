package list

//
// Implements an address list and import from/export to a CSV file. The format
// of the CSV file is one column for each AddressKey and AddressRecord element,
// with dates formatted as RFC3339 and the Ignore flag as "IGNORE" or "".
//

import (
    "os"
    "io"
    "time"
    "log"
    "strings"
    "sort"
    "errors"
    "encoding/csv"
    "regexp"
)

type AddressKey struct {
    Email       string
}

type AddressRecord struct {
    Email       string
    Name        string
    LastUsed    time.Time
    Ignore      bool
}

func (ar *AddressRecord) IgnoreStr() string {
    if ar.Ignore {
        return "IGNORE"
    } else {
        return ""
    }
}

type AddressList struct {
    addrs       map[AddressKey]*AddressRecord
    LastUpdated time.Time
    logger      *log.Logger
}

func NewAddressList(logger *log.Logger) *AddressList {
    return &AddressList{
        addrs: make(map[AddressKey]*AddressRecord),
        LastUpdated: time.Now().AddDate(-5, 0, 0),
        logger: logger,
    }
}

func (al *AddressList) ReceiveRecord(
    email, name string,
    usedTime time.Time,
) *AddressRecord {
    email = strings.ToLower(email)
    if strings.ToLower(name) == email { name = "" }

    key := AddressKey{ email }
    if ar, present := al.addrs[key]; present {
        if ar.LastUsed.Before(usedTime) {
            ar.LastUsed = usedTime
            if (name != "") { ar.Name = name }
        } else if ar.Name == "" && name != "" {
            ar.Name = name
        }
        return ar
    } else {
        ar = &AddressRecord{ email, name, usedTime, false }
        al.addrs[key] = ar
        return ar
    }
}

func (al *AddressList) ParseRecord(row []string) (*AddressRecord, error) {
    if len(row) < 4 {
        return nil, errors.New("ProcessRecord: wrong number of array args")
    }
    t, err := time.Parse(time.RFC3339, row[2])
    if err != nil { return nil, err }
    ar := al.ReceiveRecord(row[0], row[1], t)
    ar.Ignore = (row[3] == "IGNORE")
    return ar, nil
}

func (al *AddressList) Import(filename string) error {
    al.logger.Printf("list: Importing %s\n", filename)

    r, err := os.Open(filename)
    if os.IsNotExist(err) { return nil }
    if err != nil { return err }
    defer r.Close()
    cr := csv.NewReader(r)
    cr.ReuseRecord = true

    for {
        row, err := cr.Read()
        if err == io.EOF { return nil }
        if err != nil { return err }

        if _, err := al.ParseRecord(row); err != nil {
            return err
        }
    }
}

func (al *AddressList) Export(filename string) error {
    al.logger.Printf("list: Exporting %s\n", filename)

    // Save the old file
    if _, err := os.Stat(filename); err == nil {
        tempname := filename + "~"
        // Remove the temporary file if it exists
        os.Remove(tempname)
        os.Rename(filename, tempname)
        // Put back in when we're confident this program works
        // defer os.Remove(tempname)
    }

    // Set up the writer
    w, err := os.Create(filename)
    if err != nil { return err }
    defer w.Close()
    cw := csv.NewWriter(w)

    // Sort the address list
    keys := make([]AddressKey, 0, len(al.addrs))
    for k, _ := range al.addrs { keys = append(keys, k) }
    sort.Slice(keys, func(i, j int) bool {
        return al.addrs[keys[i]].LastUsed.After(al.addrs[keys[j]].LastUsed)
    })

    // Write the file
    for _, k := range keys {
        v := al.addrs[k]
        if v == nil { panic("Nil v") }
        cw.Write([]string{
            k.Email, v.Name, v.LastUsed.Format(time.RFC3339), v.IgnoreStr(),
        })
    }
    cw.Flush()
    return nil
}

func (al *AddressList) FindMatches(
    search, filename string, maxMatches int,
) ([]AddressRecord, error) {

    r, err := os.Open(filename)
    if err != nil { return nil, err }
    defer r.Close()
    cr := csv.NewReader(r)
    cr.ReuseRecord = true

    res := make([]AddressRecord, 0, maxMatches)
    s := regexp.MustCompile(`\W+`).Split(strings.ToLower(search), -1)
    searches := make([]string, 0, len(s))
    for _, str := range s { if str != "" { searches = append(searches, str) } }

    for {
        row, err := cr.Read()
        if err == io.EOF { break }
        if err != nil { return nil, err }

        rec, err := al.ParseRecord(row)
        if err != nil { return nil, err }
        if IsMatch(rec, searches) {
            res = append(res, *rec)
            if len(res) == cap(res) { break }
        }
    }
    return res, nil
}

func IsMatch(rec *AddressRecord, searches []string) bool {
    if rec.Ignore { return false }

    emailParts := strings.SplitN(strings.ToLower(rec.Email), "@", 2)
    for len(emailParts) < 2 { emailParts = append(emailParts, "") }

    for _, search := range searches {
        switch {
        case strings.Contains(strings.ToLower(rec.Name), search):
            continue
        case strings.Contains(emailParts[0], search):
            continue
        case strings.Contains("." + emailParts[1], "." + search):
            continue
        default:
            return false
        }
    }
    return true
}
