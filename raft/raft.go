package raft

//
// This is an outline of the API that raft must expose to
// the service (or tester). See comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   Create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   Start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   Each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester) in the same server.
//

import (
	"sync"
	"sync/atomic"

	"bytes"
	"cs350/labgob"
	"cs350/labrpc"

	// Import rand
	"fmt"
	"math/rand"
	"time"
)

// As each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). Set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // This peer's index into peers[]
	dead      int32               // Set by Kill()

	// Your data here (4A, 4B).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// Variables for 4A
	// 0 for follower, 1 for candidate, 2 for leader
	nodeState int
	// Might not need this
	// hasVoted bool
	// votesRecieved int
	// Used to check which node is the leader (not sure if I implement it like this or need it)
	leaderBool bool
	currTerm   int

	votedFor    int
	log         []LogStruct
	commitIndex int
	lastApplied int
	nextIndex   []int
	matchIndex  []int
	totalVotes  int

	electionTimeout int
	lastHB          time.Time

	numPeers int
	applyCh  chan ApplyMsg

	// Worker will check if there is a heartbeat in the channel after time outs
	//heartbeatChannel chan bool
	//isLeader         chan bool

}

type LogStruct struct {
	Term    int
	Command interface{}
}

// Return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	var term int
	var isleader bool
	// Your code here (4A).

	rf.mu.Lock()
	defer rf.mu.Unlock()
	// Returns if it is leader
	isleader = rf.leaderBool

	// Returns term
	term = rf.currTerm

	return term, isleader
}

// Save Raft's persistent state to stable storage, where it
// can later be retrieved after a crash and restart. See paper's
// Figure 2 for a description of what should be persistent.
func (rf *Raft) persist() {
	// Your code here (4B).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

// Restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (4B).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currTerm int
	var votedFor int
	var log []LogStruct
	if d.Decode(&currTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&log) != nil {
		fmt.Println("error")
	} else {
		rf.currTerm = currTerm
		rf.votedFor = votedFor
		rf.log = log
	}
}

// Example RequestVote RPC arguments structure.
// Field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (4A, 4B).

	Term         int // Candidate's Term
	CandidateID  int // Candidate Requesting Vote
	LastLogIndex int // Index of candidate's last log entry
	LastLogTerm  int // Term of candidate's last log entry
}

// Example RequestVote RPC reply structure.
// Field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (4A).
	Term        int  // Term for candidate to update itself
	VoteGranted bool // True means candidate recieved vote
}

// Example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (4A, 4B).

	// Leader sends out heartbeats
	// rf.currTerm has to be updated to the leader term after the candidate becomes leader
	fmt.Println("Follower ", rf.me, " getting a request to vote from ", args.CandidateID, " with term ", args.Term)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if args.Term < rf.currTerm {
		reply.Term = rf.currTerm
		reply.VoteGranted = false
		fmt.Println("Follower ", rf.me, " denied vote")
		return
	}

	// if len(rf.log)-1 > args.LastLogIndex {
	// 	fmt.Println("Length of log greater than the last log index, request vote false")
	// 	reply.VoteGranted = false
	// 	rf.votedFor = -1
	// 	return
	// }
	fmt.Println("current term: ", rf.currTerm, ", args term: ", args.Term)
	fmt.Println("Follower's ", rf.me, "log: ", rf.log)
	// if args.Term > rf.currTerm+1 {
	if args.Term > rf.currTerm {
		rf.currTerm = args.Term
		rf.votedFor = -1
		// Set to follower
		rf.nodeState = 0
		reply.VoteGranted = false
		rf.totalVotes = 0
		// The candidate is set back to follower?
		rf.persist()
		fmt.Println("Set back to follower, ", rf.me, ", args term: ", args.Term, " curr term: ", rf.currTerm)
		// return
	}

	// Might not need the first term
	// if (rf.votedFor == -1 || rf.votedFor == args.CandidateID) && (len(rf.log) == 0 || rf.log[len(rf.log)-1].Term >= args.LastLogTerm) {
	//fmt.Println("current term: ", rf.currTerm, ", args term: ", args.Term)
	fmt.Println("Last log term: ", args.LastLogTerm, ", rf.log[len(rf.log)-1].Term: ", rf.log[len(rf.log)-1].Term, ", last log index: ", args.LastLogIndex, ", len(rf.log) - 1: ", len(rf.log)-1)
	//if (rf.votedFor == -1 || rf.votedFor == args.CandidateID) && (args.LastLogTerm >= rf.log[len(rf.log)-1].Term && args.LastLogIndex >= len(rf.log)-1) {
	//if (rf.votedFor == -1 || rf.votedFor == args.CandidateID) && (len(rf.log) == 0 || rf.log[len(rf.log)-1].Term >= args.LastLogTerm) {
	// if (rf.votedFor == -1 || rf.votedFor == args.CandidateID) && (args.LastLogTerm > rf.log[len(rf.log)-1].Term ||
	// 	(args.LastLogTerm == rf.log[len(rf.log)-1].Term && args.LastLogIndex >= len(rf.log)-1)) {
	if args.LastLogTerm > rf.log[len(rf.log)-1].Term ||
		(args.LastLogTerm == rf.log[len(rf.log)-1].Term && args.LastLogIndex >= len(rf.log)-1) {
		reply.Term = rf.currTerm
		reply.VoteGranted = true
		rf.votedFor = args.CandidateID
		//rf.currTerm = args.Term
		rf.persist()
		rf.lastHB = time.Now()
		fmt.Println("Follower ", rf.me, " Granted vote for leader ", args.CandidateID)
		return
	}

	// Not sure if this is needed
	if rf.nodeState == 1 || rf.nodeState == 2 {
		rf.nodeState = 0
		rf.currTerm = args.Term
		rf.votedFor = -1
		rf.persist()
		fmt.Println("Candidates ", rf.me, " was turned back into follower")
		return
	}

	// if args.Term > rf.currTerm {
	// 	rf.currTerm = args.Term
	// 	rf.votedFor = -1
	// 	// Set to follower
	// 	rf.nodeState = 0
	// 	reply.VoteGranted = false
	// 	// The candidate is set back to follower?
	// 	fmt.Println("Set back to follower, ", rf.me, ", args term: ", args.Term, " curr term: ", rf.currTerm)
	// 	return
	// }
	//reply.VoteGranted = true
	//fmt.Println("Follower ", rf.me, " Granted vote2")
	return
}

type AppendEntriesArgs struct {
	Term         int
	LeaderID     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogStruct
	LeaderCommit int
}

type AppendEntriesReply struct {
	Success       bool
	Term          int
	ConflictIndex int
	ConflictTerm  int
}

// RPC Handler for AppendEntries
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// Args term is leader's term

	if args.Term < rf.currTerm && !(args.PrevLogIndex > len(rf.log)-1) {
		reply.Success = false
		fmt.Println("Leaders term: ", args.Term, " and follower's term: ", rf.currTerm, " NEW")
		reply.Term = rf.currTerm
		return
	} else if args.Term < rf.currTerm && (args.PrevLogIndex > len(rf.log)-1) {
		// This case is basically if the
		fmt.Println("Candidate wasn't able to convert back to follower in time")
		rf.nodeState = 0
		rf.persist()
	}

	if args.Term > rf.currTerm {
		fmt.Println("New leader sent a heartbeat that is greater change to follower: ", rf.me, " changing to term ", args.Term)
		rf.currTerm = args.Term
		rf.lastHB = time.Now()
		rf.totalVotes = 0
		rf.votedFor = -1
		rf.nodeState = 0
		reply.Success = true
		rf.persist()
	}

	// This will check if the follower changed into a candidate
	/*if rf.nodeState != 0 {
		reply.Success = false
		fmt.Println("Leaders term: ", args.Term ," and follower's term: ", rf.currTerm)
		reply.Term = rf.currTerm
		return
	}*/

	if rf.nodeState == 2 || rf.nodeState == 1 {
		// This node believes it is still a leader
		rf.nodeState = 0
		rf.leaderBool = false
		rf.lastHB = time.Now()
		rf.persist()
		fmt.Println("Node ", rf.me, " converted to follower from leader")
		return
	}

	// Still need steps 3, 4, 5 in AppendEntriesRPC
	fmt.Println("Log of ", rf.me, " :", len(rf.log))
	// Step 2 in Append Entries
	fmt.Println("len of log: ", len(rf.log), ", Prevlogindex: ", args.PrevLogIndex, ", last thing in log: ", rf.log[len(rf.log)-1])
	//if args.PrevLogIndex != -1 {
	if len(args.Entries) > 0 {

		if len(rf.log) <= args.PrevLogIndex {
			// args.PrevLogIndex--
			reply.Success = false
			reply.Term = rf.currTerm
			reply.ConflictIndex = len(rf.log)
			reply.ConflictTerm = 0
			fmt.Println("PrevLogIndex (nextIndex) was bigger than the length of the log")
			return
		}

		// if args.PrevLogIndex != -1 && args.PrevLogIndex != len(rf.log) && rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		// 	reply.ConflictTerm = rf.log[args.PrevLogIndex].Term
		// 	fmt.Println("Log terms don't match, denied entries")
		// 	for i := range rf.log {
		// 		if rf.log[i].Term == reply.ConflictTerm {
		// 			reply.ConflictIndex = i
		// 		}
		// 	}
		// 	reply.Term = rf.currTerm
		// 	reply.Success = false
		// 	return
		// }

		//if len(rf.log) > args.PrevLogIndex && rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		//if len(rf.log) != args.PrevLogIndex && rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		if rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
			reply.Success = false
			reply.Term = rf.currTerm
			for i := args.PrevLogIndex - 1; i > 0; i-- {
				if rf.log[i].Term == args.PrevLogTerm {
					reply.ConflictIndex = i
					break
				}
			}
			if reply.ConflictIndex < 1 {
				fmt.Println("Setting conflict index to 1")
				reply.ConflictIndex = 1
			}
			reply.ConflictTerm = rf.log[args.PrevLogIndex].Term
			fmt.Println("Denied append entries, conflict index ", reply.ConflictIndex)
			return
		}

		// Step 3 in Append Entries
		// if len(args.Entries) > 0 {
		// 	if len(rf.log) > args.PrevLogIndex+1 && rf.log[args.PrevLogIndex+1].Term != args.Entries[0].Term {
		// 		rf.log = rf.log[:args.PrevLogIndex+1]
		// 		fmt.Println("Step 3 in append entries failed")
		// 	}
		// }
		fmt.Println("Starting to append entries or delete")
		fmt.Println("Entries: ", args.Entries)
		if len(args.Entries) > 0 {
			firstLogIndex := args.PrevLogIndex
			entriesIndex := 0

			if firstLogIndex < len(rf.log) {
				for i := 0; i < len(args.Entries); i++ {
					fmt.Println("firstLogIndex: ", firstLogIndex, ", entriesIndex: ", entriesIndex, ", log len: ", len(rf.log))
					// if rf.log[firstLogIndex].Term != args.Entries[i].Term {
					// 	break
					// } else if firstLogIndex >= len(rf.log) || entriesIndex >= len(args.Entries) {
					// 	break
					// } else {
					// 	firstLogIndex++
					// 	entriesIndex++
					// }

					// This will check the first term where it doesn't match the term of the entries or if it goes over the log
					// thus it will go from the start of the log until the mismatch (which could be the new entries) and from
					// the entries onwards where it doesn't match
					// This would combine the steps 3 and 4 of appendEntries because it will get rid of the mismatches
					// and append the new entries
					// For the first test case this should be from log[:1] and entries[1:]
					if firstLogIndex >= len(rf.log) || entriesIndex >= len(args.Entries) {
						break
					} else if rf.log[firstLogIndex].Term != args.Entries[i].Term {
						break
					} else {
						firstLogIndex++
						entriesIndex++
					}
				}
			}
			// This combine both step 3 and 4
			// if entriesIndex < firstLogIndex {
			if entriesIndex < len(args.Entries) {
				rf.log = append(rf.log[:firstLogIndex], args.Entries[entriesIndex:]...)
				rf.persist()
				fmt.Println("Appending entries to log", len(rf.log))
			}

		}
	}

	// // Step 4 in Append Entries
	// for i, _ := range args.Entries {
	// 	rf.log = append(rf.log, args.Entries[i])
	// 	rf.lastApplied++
	// }
	// fmt.Println("Updated log after appending logs of ", rf.me, ": ", rf.log)
	// // newLog := args.Entries
	// // rf.log = newLog
	// // rf.commitIndex = args.LeaderCommit

	// Step 5 in Append Entries
	fmt.Println("Leader's commit: ", args.LeaderCommit, ", follower's commit: ", rf.commitIndex)
	if args.LeaderCommit > rf.commitIndex {
		// rf.commitIndex = min(args.LeaderCommit, len(rf.log)-1)
		if args.LeaderCommit < len(rf.log)-1 {
			rf.commitIndex = args.LeaderCommit
		} else {
			rf.commitIndex = len(rf.log) - 1
		}
		fmt.Println("Leader's commit is greater than follower's commit, follower's commit now: ", rf.commitIndex)
		rf.sendApplyChannel()
	}

	reply.Success = true
	rf.currTerm = args.Term
	reply.Term = rf.currTerm
	rf.nodeState = 0
	rf.votedFor = -1
	rf.totalVotes = 0
	// Need to add heart beats
	rf.lastHB = time.Now()
	fmt.Println("Accepted heart beat for ", rf.me, " with term ", rf.currTerm)
	//rf.heartbeatChannel <- true
	return
}

// Example code to send a RequestVote RPC to a server.
// Server is the index of the target server in rf.peers[].
// Expects RPC arguments in args. Fills in *reply with RPC reply,
// so caller should pass &reply.
//
// The types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// Look at the comments in ../labrpc/labrpc.go for more details.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

// Not sure if this is needed
// This function just calls Raft.AppendEntries
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// The service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. If this
// server isn't the leader, returns false. Otherwise start the
// agreement and return immediately. There is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. Even if the Raft instance has been killed,
// this function should return gracefully.
//
// The first return value is the index that the command will appear at
// if it's ever committed. The second return value is the current
// term. The third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := false

	// Your code here (4B).
	rf.mu.Lock()

	term = rf.currTerm
	isLeader = rf.leaderBool
	rf.mu.Unlock()
	if isLeader {
		rf.mu.Lock()
		newLog := LogStruct{
			Term:    term,
			Command: command,
		}
		rf.log = append(rf.log, newLog)
		index = len(rf.log) - 1
		rf.persist()
		fmt.Println("New leader ", rf.me, " log: ", len(rf.log))
		rf.mu.Unlock()
		for i := range rf.peers {
			if i != rf.me {
				go func(serverToSendRPCTo int) {
					rf.mu.Lock()
					hbArgs := &AppendEntriesArgs{}
					hbArgs.Term = rf.currTerm
					fmt.Println("Current nextIndex: ", rf.nextIndex)
					fmt.Println("Current leader's ", rf.me, " log len: ", len(rf.log))

					if len(rf.log) >= rf.nextIndex[serverToSendRPCTo] {
						// Initialize the index of the log entry to one less than the new ones
						hbArgs.PrevLogIndex = rf.nextIndex[serverToSendRPCTo] - 1
						fmt.Println("PrevLogIndex", hbArgs.PrevLogIndex, " from leader: ", rf.me, " to follower ", serverToSendRPCTo)
						// Initialize PrevLogTerm to term of PrevLogIndex
						hbArgs.PrevLogTerm = rf.log[hbArgs.PrevLogIndex].Term

						fmt.Println("PrevLogTerm", hbArgs.PrevLogTerm, " from leader: ", rf.me, " to follower ", serverToSendRPCTo)

						if len(rf.log) != 1 {
							hbArgs.Entries = rf.log[hbArgs.PrevLogIndex:]
							fmt.Println("Entries", hbArgs.Entries, "in leader with nextIndex: ", rf.nextIndex)
						}

						fmt.Println("leader's commit index: ", rf.commitIndex)
						hbArgs.LeaderCommit = rf.commitIndex
					} else {
						// These should be sent only when there is nothing in the log or everything has already been
						// appended/sent
						hbArgs.PrevLogIndex = -1
						hbArgs.PrevLogTerm = -1
						hbArgs.LeaderCommit = rf.commitIndex
					}

					//rf.log[]

					hbReply := &AppendEntriesReply{}
					hbReply.ConflictIndex = 0
					hbReply.ConflictTerm = 0
					rf.mu.Unlock()
					fmt.Println("Sending heart beats to ", serverToSendRPCTo, " with term ", rf.currTerm, "\n.")
					ok := rf.sendAppendEntries(serverToSendRPCTo, hbArgs, hbReply)
					fmt.Println("Sent append entries ok is ", ok, " for ", serverToSendRPCTo)
					rf.mu.Lock()
					// This is wrong because when the leader dies, it will be converted to follower
					// and its election timer will go off and convert to candidate
					// and when a new leader comes up and tries to send to old leader, it fails and becomes follower
					// if !hbReply.Success {
					// 	rf.currTerm = hbReply.Term
					// 	rf.nodeState = 0
					// 	rf.votedFor = -1
					// 	rf.nodeState = 0
					// 	rf.lastHB = time.Now()
					// 	fmt.Println("Reply from leader to follower failed convert ", rf.me, " to follower")
					// }
					if ok {
						if hbReply.Success && hbArgs.PrevLogIndex != -1 {
							// This may need to be fixed later for deletes
							rf.nextIndex[serverToSendRPCTo] = hbArgs.PrevLogIndex + 1 + len(hbArgs.Entries)
							fmt.Println("Reply success adding to nextIndex: ", rf.nextIndex)
							rf.matchIndex[serverToSendRPCTo] = hbArgs.PrevLogIndex + len(hbArgs.Entries)
							fmt.Println("Reply success adding to matchIndex: ", rf.matchIndex)
							rf.commit()
						} else if !hbReply.Success && hbArgs.PrevLogIndex != -1 {
							if hbReply.ConflictIndex > 0 || hbReply.ConflictTerm > 0 {
								found := false
								for i := range rf.log {
									if rf.log[i].Term == hbReply.ConflictTerm {
										rf.nextIndex[serverToSendRPCTo] = i + 1
										found = true
									}
								}
								if !found {
									rf.nextIndex[serverToSendRPCTo] = hbReply.ConflictIndex
								}
								fmt.Println("LEADER BACKUP UP!!!!!!!!!! to ", rf.nextIndex[serverToSendRPCTo])
							}
							// if hbReply.ConflictIndex > 0 || hbReply.ConflictTerm > 0 {
							// 	rf.nextIndex[serverToSendRPCTo] = hbReply.ConflictIndex
							// } else {
							// 	rf.nextIndex[serverToSendRPCTo] = 1
							// 	//rf.nextIndex[serverToSendRPCTo] = hbArgs.PrevLogIndex
							// }
						}

						// if len(rf.log) != 0 {
						// 	// fmt.Println("Lastapplied: ", rf.lastApplied, ", commitIndex: ", rf.commitIndex, " rf.matchIndex[serverToSendRPCTo]: ", rf.matchIndex[serverToSendRPCTo], ", rf.log[rf.lastApplied].Term: ", rf.log[rf.lastApplied - 1].Term, ", currTerm: ", rf.currTerm)
						// 	// if (rf.lastApplied > rf.commitIndex) && (rf.matchIndex[serverToSendRPCTo] >= rf.lastApplied) && (rf.log[rf.lastApplied - 1].Term == rf.currTerm) {
						// 	// 	rf.commitIndex = rf.lastApplied
						// 	// 	fmt.Println("Updated commit index to: ", rf.commitIndex)
						// 	// }
						// 	rf.commit()
						// }

						if hbReply.Term > rf.currTerm {
							fmt.Println("Follower's term is greater than leader's term. Fail leader ", rf.me)
							rf.lastHB = time.Now()
							rf.totalVotes = 0
							rf.votedFor = -1
							rf.nodeState = 0
							rf.persist()
						}
					}
					rf.mu.Unlock()
				}(i)
			}
		}
		time.Sleep(100 * time.Millisecond)
		fmt.Println("Finished leader ", rf.me)
	} else {
		return index, term, isLeader
	}

	return index, term, isLeader
}

// The tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. Your code can use killed() to
// check whether Kill() has been called. The use of atomic avoids the
// need for a lock.
//
// The issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. Any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().
		rf.mu.Lock()
		state := rf.nodeState
		rf.mu.Unlock()
		switch state {
		case 0:
			// Follower
			{
				rf.mu.Lock()
				rf.leaderBool = false
				rf.totalVotes = 0
				if time.Since(rf.lastHB) >= time.Duration(rf.electionTimeout)*time.Millisecond {
					rf.nodeState = 1
					rf.lastHB = time.Now()
					fmt.Println("Follower ", rf.me, " converted to candidate: ", rf.nodeState)
				}
				rf.mu.Unlock()
			}
		case 1:
			// Candidate
			{
				rf.mu.Lock()
				fmt.Println("Starting candidate for ", rf.me)
				candidateTime := time.Now()
				rf.leaderBool = false
				rf.totalVotes = 0
				rf.currTerm++
				rf.votedFor = rf.me
				rf.totalVotes++
				rf.lastHB = time.Now()
				// rf.mu.Unlock()
				for {
					// rf.mu.Lock()
					// shoudlIExit := false
					if time.Since(candidateTime) > time.Duration(rf.electionTimeout)*time.Millisecond {
						rf.nodeState = 0
						fmt.Println("Candidate ", rf.me, " converted back to follower: ", rf.nodeState)
						rf.persist()
						rf.mu.Unlock()
						break
					}
					rf.mu.Unlock()
					fmt.Println("Candidate ", rf.me, " going to do election")
					electionDone := rf.election()
					// rf.mu.Lock()
					if electionDone {
						rf.mu.Lock()
						fmt.Println("Election done for ", rf.me)
						// Reinitialized after election in State
						// rf.nextIndex = make([]int, len(rf.peers))
						// rf.matchIndex = make([]int, len(rf.peers))
						for i, _ := range rf.peers {
							rf.matchIndex[i] = 0
							rf.nextIndex[i] = len(rf.log)
						}
						rf.mu.Unlock()
						// shoudlIExit = true
						break
					}
					if !electionDone {
						rf.mu.Lock()
						rf.nodeState = 0
						//rf.currTerm--
						rf.votedFor = -1
						rf.persist()
						rf.lastHB = time.Now()
						fmt.Println("Election didn't finish for ", rf.me, " converted to follower")
						rf.mu.Unlock()
						break
					}
					// if shoudlIExit {
					// 	rf.mu.Unlock()
					// 	break
					// }
				}
			}
		case 2:
			// Leader
			{
				fmt.Println("Doing leader ", rf.me, "\n.")
				rf.mu.Lock()
				rf.leaderBool = true
				rf.mu.Unlock()
				for i := range rf.peers {
					if i != rf.me {
						go func(serverToSendRPCTo int) {
							rf.mu.Lock()
							hbArgs := &AppendEntriesArgs{}
							hbArgs.Term = rf.currTerm
							fmt.Println("Current nextIndex: ", rf.nextIndex)
							fmt.Println("Current leader's ", rf.me, " log len: ", len(rf.log))

							if len(rf.log) >= rf.nextIndex[serverToSendRPCTo] {
								// Initialize the index of the log entry to one less than the new ones
								hbArgs.PrevLogIndex = rf.nextIndex[serverToSendRPCTo] - 1
								fmt.Println("PrevLogIndex", hbArgs.PrevLogIndex, " from leader: ", rf.me, " to follower ", serverToSendRPCTo)
								// Initialize PrevLogTerm to term of PrevLogIndex
								hbArgs.PrevLogTerm = rf.log[hbArgs.PrevLogIndex].Term

								fmt.Println("PrevLogTerm", hbArgs.PrevLogTerm, " from leader: ", rf.me, " to follower ", serverToSendRPCTo)

								if len(rf.log) != 1 {
									hbArgs.Entries = rf.log[hbArgs.PrevLogIndex:]
									fmt.Println("Entries", hbArgs.Entries, "in leader with nextIndex: ", rf.nextIndex)
								}

								fmt.Println("leader's commit index: ", rf.commitIndex)
								hbArgs.LeaderCommit = rf.commitIndex
							} else {
								// These should be sent only when there is nothing in the log or everything has already been
								// appended/sent
								hbArgs.PrevLogIndex = -1
								hbArgs.PrevLogTerm = -1
								hbArgs.LeaderCommit = rf.commitIndex
							}

							//rf.log[]

							hbReply := &AppendEntriesReply{}
							hbReply.ConflictIndex = 0
							hbReply.ConflictTerm = 0
							rf.mu.Unlock()
							fmt.Println("Sending heart beats to ", serverToSendRPCTo, " with term ", rf.currTerm, "\n.")
							ok := rf.sendAppendEntries(serverToSendRPCTo, hbArgs, hbReply)
							fmt.Println("Sent append entries ok is ", ok, " for ", serverToSendRPCTo)
							rf.mu.Lock()
							// This is wrong because when the leader dies, it will be converted to follower
							// and its election timer will go off and convert to candidate
							// and when a new leader comes up and tries to send to old leader, it fails and becomes follower
							// if !hbReply.Success {
							// 	rf.currTerm = hbReply.Term
							// 	rf.nodeState = 0
							// 	rf.votedFor = -1
							// 	rf.nodeState = 0
							// 	rf.lastHB = time.Now()
							// 	fmt.Println("Reply from leader to follower failed convert ", rf.me, " to follower")
							// }
							if ok {
								if hbReply.Success && hbArgs.PrevLogIndex != -1 {
									// This may need to be fixed later for deletes
									rf.nextIndex[serverToSendRPCTo] = hbArgs.PrevLogIndex + 1 + len(hbArgs.Entries)
									fmt.Println("Reply success adding to nextIndex: ", rf.nextIndex)
									rf.matchIndex[serverToSendRPCTo] = hbArgs.PrevLogIndex + len(hbArgs.Entries)
									fmt.Println("Reply success adding to matchIndex: ", rf.matchIndex)
									rf.commit()
								} else if !hbReply.Success && hbArgs.PrevLogIndex != -1 {
									if hbReply.ConflictIndex > 0 || hbReply.ConflictTerm > 0 {
										found := false
										for i := range rf.log {
											if rf.log[i].Term == hbReply.ConflictTerm {
												rf.nextIndex[serverToSendRPCTo] = i + 1
												found = true
											}
										}
										if !found {
											rf.nextIndex[serverToSendRPCTo] = hbReply.ConflictIndex
										}
										fmt.Println("LEADER BACKUP UP!!!!!!!!!! to ", rf.nextIndex[serverToSendRPCTo])
									}
									// if hbReply.ConflictIndex > 0 || hbReply.ConflictTerm > 0 {
									// 	rf.nextIndex[serverToSendRPCTo] = hbReply.ConflictIndex
									// } else {
									// 	rf.nextIndex[serverToSendRPCTo] = 1
									// 	//rf.nextIndex[serverToSendRPCTo] = hbArgs.PrevLogIndex
									// }
								}

								// if len(rf.log) != 0 {
								// 	// fmt.Println("Lastapplied: ", rf.lastApplied, ", commitIndex: ", rf.commitIndex, " rf.matchIndex[serverToSendRPCTo]: ", rf.matchIndex[serverToSendRPCTo], ", rf.log[rf.lastApplied].Term: ", rf.log[rf.lastApplied - 1].Term, ", currTerm: ", rf.currTerm)
								// 	// if (rf.lastApplied > rf.commitIndex) && (rf.matchIndex[serverToSendRPCTo] >= rf.lastApplied) && (rf.log[rf.lastApplied - 1].Term == rf.currTerm) {
								// 	// 	rf.commitIndex = rf.lastApplied
								// 	// 	fmt.Println("Updated commit index to: ", rf.commitIndex)
								// 	// }
								// 	rf.commit()
								// }

								if hbReply.Term > rf.currTerm {
									fmt.Println("Follower's term is greater than leader's term. Fail leader ", rf.me)
									rf.lastHB = time.Now()
									rf.totalVotes = 0
									rf.votedFor = -1
									rf.nodeState = 0
									rf.persist()
								}
							}
							rf.mu.Unlock()
						}(i)
					}
				}
				time.Sleep(100 * time.Millisecond)
				fmt.Println("Finished leader ", rf.me)
			}
		}
	}
}

func (rf *Raft) commit() bool {
	// This should only be called in leader
	fmt.Println("Trying to commit with commit index of ", rf.commitIndex, " and length of log is ", len(rf.log))
	// This is the bottom step in Rules for servers
	for n := rf.commitIndex + 1; n < len(rf.log); n++ {
		fmt.Println("n: ", n)
		// The leader also has the same match index as the follower
		totalMajority := 1
		for i, _ := range rf.matchIndex {
			if rf.matchIndex[i] > n && rf.log[n].Term == rf.currTerm && i != rf.me {
				fmt.Println("Total majority is added")
				totalMajority++
			}
		}
		// This may be sent more than once???
		if totalMajority > (len(rf.matchIndex) / 2) {
			rf.commitIndex = n
			fmt.Println("New commit index: ", rf.commitIndex)
			fmt.Println("Sending to apply channel from commit")
			rf.sendApplyChannel()
		}
	}
	return true
}

func (rf *Raft) sendApplyChannel() bool {
	// fmt.Println("Last applied: ", rf.lastApplied, " and commit index: ", rf.commitIndex)
	// if rf.lastApplied > rf.commitIndex {
	// 	for i := rf.commitIndex; i < rf.lastApplied; i++ {
	// 		rf.applyCh <- ApplyMsg{
	// 			CommandValid: true,
	// 			Command:      rf.log[i].Command,
	// 			CommandIndex: rf.commitIndex,
	// 		}
	// 		fmt.Println("Sent log ", rf.log[rf.lastApplied-1], " to apply channel")
	// 	}
	// 	rf.commitIndex = rf.lastApplied
	// 	fmt.Println("New commit index: ", rf.commitIndex)
	// }
	if rf.lastApplied < rf.commitIndex {
		fmt.Println("Node: ", rf.me, ", Last applied: ", rf.lastApplied, " and commit index: ", rf.commitIndex)
		// start := rf.commitIndex
		// for i := start; i < len(rf.log); i++ {
		start := rf.lastApplied + 1
		for i := start; i < rf.commitIndex+1; i++ {
			rf.applyCh <- ApplyMsg{
				CommandValid: true,
				Command:      rf.log[i].Command,
				CommandIndex: i,
			}
			fmt.Println("Sent log ", rf.log[i], " to apply channel")

			rf.lastApplied++
			// if i > rf.commitIndex {
			// 	rf.commitIndex = i
			// }
		}
	}
	// This should only work if there is only one item that was added to the log
	// if start == rf.commitIndex {
	// 	rf.applyCh <- ApplyMsg{
	// 		CommandValid: true,
	// 		Command:      rf.log[rf.commitIndex].Command,
	// 		CommandIndex: rf.lastApplied + 1,
	// 	}
	// 	fmt.Println("Sent log ", rf.log[rf.commitIndex], " to apply channel for lastapplied == commit")
	// 	rf.commitIndex++
	// 	rf.lastApplied++
	// }
	fmt.Println("New last applied: ", rf.lastApplied, " and new commit index: ", rf.commitIndex)
	return true
}

func (rf *Raft) election() bool {
	for i, _ := range rf.peers {
		if i != rf.me {
			go func(i int) {
				rf.mu.Lock()
				fmt.Println("Election func creation with peer ", i)
				// Send request vote to all followers
				voteArgs := &RequestVoteArgs{}
				voteArgs.CandidateID = rf.me
				voteArgs.Term = rf.currTerm
				// These are set to 0 for now
				if len(rf.log) > 0 {
					voteArgs.LastLogIndex = len(rf.log) - 1
					voteArgs.LastLogTerm = rf.log[len(rf.log)-1].Term
				} else {
					voteArgs.LastLogIndex = 0
					voteArgs.LastLogTerm = 0
				}

				voteReply := &RequestVoteReply{}
				fmt.Println("Before sending request vote")
				rf.mu.Unlock()
				rf.sendRequestVote(i, voteArgs, voteReply)
				fmt.Println("After sending request vote")
				rf.mu.Lock()

				if voteReply.Term > rf.currTerm {
					rf.currTerm = 0
					fmt.Println("Vote reply's term was bigger than candidate's term changing ", rf.me, " back to follower")
				}

				if voteReply.VoteGranted && rf.nodeState != 2 {
					rf.totalVotes++
					fmt.Println("Adding vote. Total: ", rf.totalVotes)
					if rf.totalVotes > len(rf.peers)/2 && rf.nodeState != 2 {
						fmt.Println("becoming leader")
						rf.nodeState = 2
						rf.persist()
						fmt.Println("Candidate ", rf.me, " is converted to leader ", rf.nodeState, " with term ", rf.currTerm)
						// Send empty heartbeats
						for i, _ := range rf.peers {
							if i != rf.me {
								//rf.mu.Lock()
								emptyArgs := &AppendEntriesArgs{}
								emptyReply := &AppendEntriesReply{}
								emptyArgs.Term = rf.currTerm
								//rf.mu.Unlock()
								fmt.Println("Sending empty heart beats to ", i)
								go func(i int) {
									rf.sendAppendEntries(i, emptyArgs, emptyReply)
								}(i)
							}
						}
					}
				}

				rf.mu.Unlock()
			}(i)
		}
	}
	//rf.mu.Lock()
	time.Sleep(100 * time.Millisecond)
	fmt.Println("Election loop exits")
	//rf.mu.Unlock()
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.nodeState == 2 {
		return true
	}
	return false
}

// The service or tester wants to create a Raft server. The ports
// of all the Raft servers (including this one) are in peers[]. This
// server's port is peers[me]. All the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (4A, 4B).
	// Initialization for 4A
	// Node state set to follower
	rf.nodeState = 0
	/* rf.hasVoted = false
	rf.votesRecieved = 0 */
	rf.leaderBool = false
	rf.currTerm = 0

	// -1 represents null for votedFor
	rf.votedFor = -1
	rf.log = make([]LogStruct, 1)

	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.nextIndex = make([]int, len(rf.peers))
	rf.matchIndex = make([]int, len(rf.peers))

	rf.totalVotes = 0

	//rf.heartbeatChannel = make(chan bool)
	//rf.isLeader = make(chan bool)

	rf.electionTimeout = rand.Intn(500) + 500

	rf.numPeers = len(peers)
	rf.lastHB = time.Now()

	rf.applyCh = applyCh

	fmt.Println("Created server: ", rf.me)

	// initialize from state persisted before a crash.
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections.
	go rf.ticker()

	return rf
}
