function showComments() {
    var x = document.getElementsByClassName("displayCommentsDiv")[0];
    if (x.style.display !== "block") {
        x.style.display = "block";
    } else {
        x.style.display = "none";
    }
}

/*
function showReactions(i) {
    var x = document.getElementsByClassName("displayReactionsDiv")[0];
        if (x.style.display !== "block") {
            x.style.display = "block";
        } else {
            x.style.display = "none";
        }
} 
*/
function showReactions() {
    var x = document.getElementsByClassName("displayReactionsDiv");
    for (let i=0;i<x.length;i++){
        if (x[i].style.display !== "block") {
            x[i].style.display = "block";
        } else {
            x[i].style.display = "none";
        }
    }
} 

function showCommentReactions() {
    var x = document.getElementsByClassName("displayCommentReactionsDiv");
    for (let i=0;i<x.length;i++){
        if (x[i].style.display !== "block") {
            x[i].style.display = "block";
        } else {
            x[i].style.display = "none";
        }
    }
}

function showPrivMsgWriteBox(){
    var checkBox = document.getElementById("privMsgCheckbox");
    var text = document.getElementById("privMsgWriteBox");
    if (checkBox.checked == true){
        text.style.display = "block";
    } else {
        text.style.display = "none";
    }
}

function showFollowers() {
    var x = document.getElementsByClassName("followersList")[0];
    if (x.style.display === "none") {
        x.style.display = "block";
    } else {
        x.style.display = "none";
    }
}

function showFollowees() {
    var x = document.getElementsByClassName("followeesList")[0];
    if (x.style.display === "none") {
        x.style.display = "block";
    } else {
        x.style.display = "none";
    }
}

function showPrivMsgWriteBox(){
    var checkBox = document.getElementById("privMsgCheckbox");
    var text = document.getElementById("privMsgWriteBox");
    if (checkBox.checked == true){
        text.style.display = "block";
    } else {
        text.style.display = "none";
    }
}