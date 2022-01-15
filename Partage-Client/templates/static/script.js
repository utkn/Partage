function choosePrivateVisibility() {
    let privBox = document.getElementById("privMsgWriteBox")
    let publicBox = document.getElementById("publicMsgWriteBox")
    let privBtn = document.getElementById("privatePostButton")
    let publicBtn = document.getElementById("publicPostButton")
    publicBox.style.display = "none"
    privBox.style.display = "block"
    publicBtn.classList.remove("pure-button-active")
    privBtn.classList.add("pure-button-active")
}

function choosePublicVisibility() {
    let privBox = document.getElementById("privMsgWriteBox")
    let publicBox = document.getElementById("publicMsgWriteBox")
    let privBtn = document.getElementById("privatePostButton")
    let publicBtn = document.getElementById("publicPostButton")
    publicBox.style.display = "block"
    privBox.style.display = "none"
    privBtn.classList.remove("pure-button-active")
    publicBtn.classList.add("pure-button-active")
}
function toggleDisplays(toOpenID, toCloseID) {
    let x = document.getElementById(toOpenID);
    if (x.style.display === "none") {
        let y = document.getElementById(toCloseID);
        y.style.display = "none";
        x.style.display = "block";
    } else {
        x.style.display = "none";
    }
}

function toggleDisplay(toOpenID) {
    let x = document.getElementById(toOpenID);
    if (x.style.display === "none") {
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