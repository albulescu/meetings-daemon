package main

import (
    "log"
    "flag"
    "fmt"
    "time"
    "bytes"
    "os"
    "path/filepath"
    "gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
    "github.com/alexjlockwood/gcm"
    "gopkg.in/ini.v1"
)

const (
    STATUS_SCHEDULED    = 1
    STATUS_ACTIVE       = 2
    STATUS_COMPLETE     = 3
    STATUS_CANCELED     = 4
    STATUS_ZOMBIE       = 5
)

type Device struct {
    Id bson.ObjectId                `bson:"_id,omitempty"`
    Token string                    `bson:"token,omitempty"`
    Name string                     `bson:"name,omitempty"`
    Platform string                 `bson:"platform,omitempty"`
}

type User struct {
    Id bson.ObjectId                `bson:"_id,omitempty"`
    FirstName string                `bson:"firstName,omitempty"`
    LastName string                 `bson:"lastName,omitempty"`
    Devices []Device                `bson:"devices,omitempty"`
}

type Meeting struct {
    Id bson.ObjectId                `bson:"_id,omitempty"`
    Goal string                     `bson:"goal"`
    Participants []bson.ObjectId    `bson:"participants"`
    Owner bson.ObjectId             `bson:"owner,omitempty"`
    Company bson.ObjectId           `bson:"company,omitempty"`
    Room bson.ObjectId              `bson:"room,omitempty"`
    StartTime time.Time             `bson:"start_time,omitempty"`
    EndTime time.Time               `bson:"end_time,omitempty"`
}

func (m Meeting) String() string {
        return fmt.Sprintf("[%s]@[%s]", m.Id.Hex(), m.Goal);
}

var (
    session *mgo.Session;
    collection *mgo.Collection;
    db *mgo.Database;
    config *ini.File
)

func connect( info *mgo.DialInfo ) (err error) {

    sess, err := mgo.DialWithInfo(info)

    if err != nil {
        return err
    }

    sess.SetMode(mgo.Monotonic, true)

    session = sess;
    db = session.DB("om");
    collection = db.C("meetings");
    return nil;
}

func meetings(query bson.M) (error,[]Meeting) {

    var meetings = []Meeting{};

    err := collection.Find(query).All(&meetings)

    if err != nil {
        return err, nil
    }

    return nil, meetings
}

func devices( meeting Meeting ) []string {

    var devices []string

    for _,participant := range meeting.Participants {
        var user = User{};
        db.C("users").FindId(participant).One(&user)
        for _, device := range user.Devices {
            devices = append(devices, device.Token)
        }
    }

    return devices
}

func notify( meeting Meeting, status int ) {

    data := map[string]interface{}{
        "action": "meeting_started",
        "meeting_id": meeting.Id.Hex(),
        "meeting_goal": meeting.Goal,
    }

    devices := devices(meeting)
    msg := gcm.NewMessage(data, devices...)

    cfg := config.Section("gcm");

    apikey := cfg.Key("apikey").String();

    sender := &gcm.Sender{ApiKey: apikey}

    _, err := sender.Send(msg, cfg.Key("retries").MustInt(3))

    if err != nil {
        log.Fatal("Failed to send message:", err)
    } else {
        log.Print("Sent push notification to ", len(devices), " devices");
    }
}

func status(meeting Meeting, status int) {

    update:=bson.M{ "$set" : bson.M{"status":status}};
    err := collection.Update(bson.M{"_id":meeting.Id},update);

    if( err == nil) {
        log.Print("Meeting ", meeting, " status changed to ", status);
    } else {
        log.Fatal("Fail to update meeting");
    }

    notify(meeting, status);
}

func check( complete chan<- int ) {

    log.Print("Checking...");

    startedQuery := bson.M{
        "status": STATUS_SCHEDULED,
        "start_time": bson.M{
            "$lte" : time.Now(),
        },
    };

    err, started := meetings(startedQuery);

    if( err == nil ) {

        if len(started) > 0 {
            for _,meeting := range started {
                go status(meeting, STATUS_ACTIVE)
            }
        } else {
            log.Print("No entries")
        }
    }
/*
    completedQuery := bson.M{};
    err, completed := meetings(completedQuery);
    if(err == nil) {
        for _,meeting := range completed {
            go status(meeting, STATUS_COMPLETE);
        }
    }
*/
    complete <- 1
}

func onError(err error, messages ...string) {
    if( err != nil ) {

        var buffer bytes.Buffer

        for index,message := range messages {
            if index == 0 {
                buffer.WriteString("[ERROR] ")
            }
            buffer.WriteString(message)
            buffer.WriteString(" ")
        }

        log.Fatal(buffer.String());
        os.Exit(1);
    }
}

func main() {

    log.Print("Starting...");

    var configFileValue = flag.String("config", "/etc/omeetings.ini", "OMeetings config file");

    var flagUser = flag.String("username", "", "Mongo username")
    var flagPass = flag.String("password", "", "Mongo password")
    var flagHost = flag.String("host", "", "Mongo hostname")
    var flagDatabase = flag.String("database", "", "Mongo database")
    var flagSource = flag.String("source", "", "Mongo user source")

    flag.Parse();

    configFile, err := filepath.Abs(*configFileValue)

    onError(err, "Fail to get absolute path from ", *configFileValue)

    config, err = ini.Load(configFile)

    onError(err, "Config file not exist", configFile);

    durationIntervalString := config.Section("").Key("interval").MustString("5s")

    mongo := config.Section("mongo");

    var user = mongo.Key("username").MustString(*flagUser)
    var pass = mongo.Key("password").MustString(*flagPass)
    var host = mongo.Key("host").MustString(*flagHost)
    var database = mongo.Key("database").MustString(*flagDatabase)
    var source = mongo.Key("source").MustString(*flagSource)

    if *flagUser != "" {
        user = *flagUser;
    }

    if *flagPass != "" {
        pass = *flagPass;
    }

    if *flagHost != "" {
        host = *flagHost;
    }

    if *flagDatabase != "" {
        database = *flagDatabase;
    }

    if *flagSource != "" {
        source = *flagSource;
    }

    log.Print("Delay duration is: ", durationIntervalString)

    intervalDuration,err := time.ParseDuration(durationIntervalString)

    onError(err, "Fail to parse interval value");

    timeout, error := time.ParseDuration(mongo.Key("timeout").MustString("10s"))

    if error != nil {
       timeout = 10 * time.Second;
    }

    dialInfo := &mgo.DialInfo{
        Addrs:    []string{host},
        Timeout:  timeout,
        Database: database,
        Username: user,
        Password: pass,
        Source: source,
    }

    log.Print("Connecting to mongo: ", host);

    err = connect( dialInfo );

    if( err != nil ) {
        log.Fatal("Fail to connect to mongo database");
        os.Exit(1);
    } else {
        log.Print("Connected!");
    }

    complete := make(chan int);

    for {
        go check( complete ); <- complete
        time.Sleep(intervalDuration);
    }
}
