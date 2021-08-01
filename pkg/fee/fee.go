// Package fee provides access to the club fee of the Buddy System.
package fee

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/kmu-kcc/buddy-backend/config"
	"github.com/kmu-kcc/buddy-backend/pkg/member"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrDuplicatedFee = errors.New("duplicated fee")

// Fee represents a club fee state.
type Fee struct {
	Year     int                  `json:"year,string" bson:"year"`
	Semester int                  `json:"semester,string" bson:"semester"`
	Amount   int                  `json:"amount,string" bson:"amount"`
	Logs     []primitive.ObjectID `json:"logs" bson:"logs"`
}

// New returns a new club fee.
func New(year, semester, amount int) *Fee {
	return &Fee{
		Year:     year,
		Semester: semester,
		Amount:   amount,
		Logs:     []primitive.ObjectID{},
	}
}

// Create creates a new fees history.
//
// NOTE:
//
// It is privileged operation:
//	Only the club managers can access to this operation.
func Create(year, semester, amount int) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))
	if err != nil {
		return
	}

	collection := client.Database("club").Collection("fees")
	fee := new(Fee)

	if err = collection.FindOne(ctx, bson.D{
		bson.E{Key: "year", Value: year},
		bson.E{Key: "semester", Value: semester},
	}).Decode(fee); err != mongo.ErrNoDocuments {
		if err = client.Disconnect(ctx); err != nil {
			return
		}
		return ErrDuplicatedFee
	}

	if _, err = collection.InsertOne(ctx, New(year, semester, amount)); err != nil {
		return
	}

	return client.Disconnect(ctx)
}

// Submit creates fees payment application log.
//
// NOTE:
//
// It is member-limited operation:
//	Only the authenticated members can access to this operation.
func Submit(memberID string, year, semester, amount int) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))
	if err != nil {
		return
	}

	feeCollection := client.Database("club").Collection("fees")
	logCollection := client.Database("club").Collection("logs")

	log := NewLog(memberID, "unapproved", amount)

	if _, err = logCollection.InsertOne(ctx, log); err != nil {
		return
	}

	if _, err = feeCollection.UpdateOne(ctx,
		bson.D{
			bson.E{Key: "year", Value: year},
			bson.E{Key: "semester", Value: semester},
		},
		bson.D{
			bson.E{Key: "$push", Value: bson.D{
				bson.E{Key: "logs", Value: log.ID},
			}},
		}); err != nil {
		return
	}
	return client.Disconnect(ctx)
}

// Amount finds log by year and semester, and returns the sum of all amounts using memberID and type.
//
// NOTE:
//
// It is member-limited operation:
//	Only the authenticated members can access to this operation.
func Amount(year, semester int, memberID string) (sum int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))
	if err != nil {
		return
	}

	fee := new(Fee)
	log := new(Log)

	if err = client.Database("club").
		Collection("fees").
		FindOne(ctx, bson.M{
			"year":     year,
			"semester": semester,
		}).Decode(fee); err != nil {
		return
	}

	cur, err := client.Database("club").
		Collection("logs").
		Find(ctx, bson.M{
			"member_id": memberID,
			"type":      "approved",
		})
	if err != nil {
		return
	}

	for cur.Next(ctx) {
		if err = cur.Decode(log); err != nil {
			return
		}
		sum += log.Amount
	}

	if err = cur.Close(ctx); err != nil {
		return
	}

	return sum, client.Disconnect(ctx)
}

// Dones returns the list of members who submitted the fee in specific year and semester.
//
// NOTE:
//
// It is privileged operation:
//	Only the club managers can access to this operation.
func Dones(year, semester int) (members member.Members, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))
	if err != nil {
		return
	}

	fee := new(Fee)
	log := new(Log)
	memb := new(member.Member)

	if err = client.Database("club").
		Collection("fees").
		FindOne(ctx, bson.M{"year": year, "semester": semester}).Decode(fee); err != nil {
		return
	}

	filter := func() bson.D {
		arr := make(bson.A, len(fee.Logs))
		for idx, logID := range fee.Logs {
			arr[idx] = logID
		}
		return bson.D{
			bson.E{Key: "_id", Value: bson.D{bson.E{Key: "$in", Value: arr}}},
			bson.E{Key: "type", Value: "approved"},
		}
	}()

	cur, err := client.Database("club").Collection("logs").Find(ctx, filter)
	if err != nil {
		return
	}

	amounts := make(map[string]int)

	for cur.Next(ctx) {
		if err = cur.Decode(log); err != nil {
			return
		}
		amounts[log.MemberID] += log.Amount
	}
	if err = cur.Close(ctx); err != nil {
		return
	}

	filter = func() bson.D {
		arr := bson.A{}
		for membID, amount := range amounts {
			if fee.Amount <= amount {
				arr = append(arr, membID)
			}
		}
		return bson.D{bson.E{Key: "id", Value: bson.D{bson.E{Key: "$in", Value: arr}}}}
	}()

	cur, err = client.Database("club").Collection("members").Find(ctx, filter)
	if err != nil {
		return
	}

	for cur.Next(ctx) {
		if err = cur.Decode(memb); err != nil {
			return
		}
		members = append(members, *memb)
	}
	if err = cur.Close(ctx); err != nil {
		return
	}
	return members, client.Disconnect(ctx)
}

// Yets returns the list of members who have not yet submitted the fee in specific year and semester.
//
// NOTE:
//
// It is privileged operation:
//	Only the club managers can access to this operation.
func Yets(year, semester int) (members member.Members, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))
	if err != nil {
		return
	}

	fee := new(Fee)
	log := new(Log)
	memb := new(member.Member)

	if err = client.Database("club").
		Collection("fees").
		FindOne(ctx, bson.M{"year": year, "semester": semester}).Decode(fee); err != nil {
		return
	}

	filter := func() bson.D {
		arr := make(bson.A, len(fee.Logs))
		for idx, logID := range fee.Logs {
			arr[idx] = logID
		}
		return bson.D{
			bson.E{Key: "_id", Value: bson.D{bson.E{Key: "$in", Value: arr}}},
			bson.E{Key: "type", Value: "approved"},
		}
	}()

	cur, err := client.Database("club").Collection("logs").Find(ctx, filter)
	if err != nil {
		return
	}

	amounts := make(map[string]int)

	for cur.Next(ctx) {
		if err = cur.Decode(log); err != nil {
			return
		}
		amounts[log.MemberID] += log.Amount
	}
	if err = cur.Close(ctx); err != nil {
		return
	}

	filter = func() bson.D {
		arr := bson.A{}
		for membID, amount := range amounts {
			if amount < fee.Amount {
				arr = append(arr, membID)
			}
		}
		return bson.D{bson.E{Key: "id", Value: bson.D{bson.E{Key: "$in", Value: arr}}}}
	}()

	cur, err = client.Database("club").Collection("members").Find(ctx, filter)
	if err != nil {
		return
	}

	for cur.Next(ctx) {
		if err = cur.Decode(memb); err != nil {
			return
		}
		members = append(members, *memb)
	}
	if err = cur.Close(ctx); err != nil {
		return
	}
	return members, client.Disconnect(ctx)
}

// All returns the all club fee logs.
//
// NOTE:
//
// It is member-limited operation:
//	Only the authenticated members can access to this operation.
func All(year, semester int) (logs Logs, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))
	if err != nil {
		return
	}

	fee := new(Fee)
	log := new(Log)

	if err = client.Database("club").
		Collection("fees").
		FindOne(ctx, bson.D{
			bson.E{Key: "year", Value: year},
			bson.E{Key: "semester", Value: semester},
		}).Decode(fee); err != nil {
		return
	}

	filter := func() bson.D {
		arr := make(bson.A, len(fee.Logs))
		for idx, logID := range fee.Logs {
			arr[idx] = logID
		}
		return bson.D{
			bson.E{Key: "_id", Value: bson.D{bson.E{Key: "$in", Value: arr}}},
			bson.E{Key: "$or", Value: bson.A{
				bson.D{bson.E{Key: "type", Value: "approved"}},
				bson.D{bson.E{Key: "type", Value: "direct"}}}}}
	}()

	cur, err := client.Database("club").Collection("logs").Find(ctx, filter)
	if err != nil {
		return
	}

	for cur.Next(ctx) {
		if err = cur.Decode(log); err != nil {
			return
		}
		logs = append(logs, *log)
	}
	if err = cur.Close(ctx); err != nil {
		return
	}

	sort.Slice(logs, func(i, j int) bool { return logs[i].UpdatedAt < logs[j].UpdatedAt })

	return logs, client.Disconnect(ctx)
}

// Approve approves the submission request of ids.
//
// Note :
//
// This is privileged operation:
// 	Only the club managers can access to this operation.
func Approve(ids []primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))

	if err != nil {
		return err
	}

	// update logs to be approved
	filter := func() bson.D {
		arr := make(bson.A, len(ids))
		for idx, id := range ids {
			arr[idx] = id

		}
		return bson.D{bson.E{Key: "_id", Value: bson.D{bson.E{Key: "$in", Value: arr}}}}
	}()

	if _, err = client.Database("club").
		Collection("logs").
		UpdateMany(
			ctx,
			filter,
			bson.D{
				bson.E{Key: "$set", Value: bson.D{
					bson.E{Key: "type", Value: "approved"},
					bson.E{Key: "updated_at", Value: time.Now().Unix()}}}}); err != nil {
		return err
	}

	return client.Disconnect(ctx)
}

// Reject rejects the submission request of ids.
//
// Note :
//
// This is privileged operation:
// 	Only the club managers can access to this operation
func Reject(year, semester int, ids []primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))
	if err != nil {
		return err
	}

	for _, id := range ids {
		if _, err := client.Database("club").Collection("fees").UpdateOne(ctx, bson.M{
			"year":     year,
			"semester": semester,
		},
			bson.D{
				bson.E{Key: "$pull", Value: bson.D{
					bson.E{Key: "logs", Value: id},
				},
				},
			}); err != nil {
			return err
		}
		if _, err := client.Database("club").Collection("logs").DeleteOne(ctx, bson.M{"_id": id}); err != nil {
			return err
		}
	}
	return client.Disconnect(ctx)
}

// Deposit makes a new log with amount and append it to fee with Year  of year, Semester of semester
//
// Note :
//
// This is privileged operation:
// 	Only the club managers can access to this operation
func Deposit(year, semester, amount int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoURI))
	if err != nil {
		return err
	}

	deposit := NewLog("", "direct", amount)

	if _, err := client.Database("club").Collection("fees").UpdateOne(ctx,
		bson.D{
			bson.E{Key: "year", Value: year},
			bson.E{Key: "semester", Value: semester},
		},
		bson.D{
			bson.E{Key: "$push", Value: bson.D{
				bson.E{Key: "logs", Value: deposit.ID},
			}},
		}); err != nil {
		return err
	}
	client.Database("club").Collection("logs").InsertOne(ctx, deposit)
	return client.Disconnect(ctx)
}