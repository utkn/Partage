function showComments() {
    var x = document.getElementsByClassName("displayCommentsDiv")[0];
    if (x.style.display !== "block") {
        x.style.display = "block";
    } else {
        x.style.display = "none";
    }
}

function showReactions() {
    var x = document.getElementsByClassName("displayReactionsDiv")[0];
    if (x.style.display !== "block") {
        x.style.display = "block";
    } else {
        x.style.display = "none";
    }
}

function showCommentReactions() {
    var x = document.getElementsByClassName("displayReactionsDiv");
    if (x.style.display !== "block") {
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

function showFollowers() {
    var x = document.getElementsByClassName("followersList");
    if (x.style.display === "none") {
        x.style.display = "block";
    } else {
        x.style.display = "none";
    }
}

function showFollowees() {
    var x = document.getElementsByClassName("followeesList");
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