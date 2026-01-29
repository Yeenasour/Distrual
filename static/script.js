text = document.querySelector("h1");

text.addEventListener("click", () => {
	text.classList.toggle("redtext");
	text.classList.toggle("bluetext")
})