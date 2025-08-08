package main

// SampleContent represents a sample content item
type SampleContent struct {
	Title   string
	Content string
}

// GetSampleData returns sample content for testing hybrid search
func GetSampleData() []SampleContent {
	return []SampleContent{
		{
			Title: "Go Programming Language",
			Content: `Go is a programming language developed by Google. It was designed by Robert Griesemer, 
Rob Pike, and Ken Thompson. Go is syntactically similar to C, but with memory safety, garbage collection, 
structural typing, and CSP-style concurrency. Go was designed at Google in 2007 to improve programming 
productivity in an era of multicore, networked machines and large codebases. The designers wanted to address 
criticism of other languages in use at Google, but keep their useful characteristics.`,
		},
		{
			Title: "Machine Learning Fundamentals",
			Content: `Machine learning is a subset of artificial intelligence that focuses on the use of data 
and algorithms to imitate the way humans learn, gradually improving accuracy. Machine learning is an important 
component of data science. Through the use of statistical methods, algorithms are trained to make classifications 
or predictions, uncovering key insights within data mining projects. These insights subsequently drive decision 
making within applications and businesses.`,
		},
		{
			Title: "Vector Databases Explained",
			Content: `Vector databases are specialized databases designed to store, index, and query high-dimensional 
vectors efficiently. They are particularly useful for applications involving machine learning, natural language 
processing, and similarity search. Unlike traditional databases that store structured data in rows and columns, 
vector databases store mathematical representations of data as vectors in high-dimensional space. This makes them 
ideal for semantic search, recommendation systems, and AI applications.`,
		},
		{
			Title: "SQLite Database System",
			Content: `SQLite is a C-language library that implements a small, fast, self-contained, high-reliability, 
full-featured, SQL database engine. SQLite is the most used database engine in the world. SQLite is built into 
all mobile phones and most computers and comes bundled inside countless other applications that people use every day. 
The SQLite file format is stable, cross-platform, and backwards compatible and the developers pledge to keep it 
that way through the year 2050.`,
		},
		{
			Title: "Semantic Search Technology",
			Content: `Semantic search denotes search with meaning, as distinguished from lexical search where the 
search engine looks for literal matches of the query words or variants of them, without understanding the overall 
meaning of the query. Semantic search uses vector embeddings to understand the intent and contextual meaning of 
search queries rather than just matching keywords. This enables more accurate and relevant search results by 
understanding the relationships between concepts.`,
		},
		{
			Title: "Hybrid Search Strategies",
			Content: `Hybrid search combines multiple search methodologies to provide more comprehensive and accurate 
results. By combining traditional keyword-based search with semantic vector search, hybrid approaches can capture 
both exact matches and conceptual similarities. This dual approach leverages the precision of keyword search for 
specific terms while using semantic search to find contextually relevant content that might not contain the exact 
query terms.`,
		},
		{
			Title: "Embedding Models Overview",
			Content: `Embedding models transform text, images, or other data into dense vector representations that 
capture semantic meaning. These models are trained on large datasets to understand relationships between concepts. 
Popular embedding models include OpenAI's text-embedding-ada-002, Google's Universal Sentence Encoder, and various 
open-source alternatives like SentenceTransformers. The quality of embeddings directly impacts the performance of 
semantic search and other vector-based applications.`,
		},
		{
			Title: "Natural Language Processing",
			Content: `Natural Language Processing (NLP) is a branch of artificial intelligence that helps computers 
understand, interpret and manipulate human language. NLP draws from many disciplines, including computer science 
and computational linguistics, in its pursuit to fill the gap between human communication and computer understanding. 
Modern NLP techniques often rely on deep learning models and large language models to achieve state-of-the-art 
performance in tasks like translation, sentiment analysis, and text generation.`,
		},
		{
			Title: "Information Retrieval Systems",
			Content: `Information retrieval is the science of searching for information in a document, searching for 
documents themselves, and searching for metadata that describes data. The most familiar example of information 
retrieval is web search engines, but the field encompasses much more. Traditional IR systems rely on term frequency 
and inverse document frequency (TF-IDF) scoring, while modern systems increasingly incorporate neural networks and 
vector representations for improved relevance.`,
		},
		{
			Title: "Database Performance Optimization",
			Content: `Database performance optimization involves various techniques to improve the speed and efficiency 
of database operations. This includes proper indexing strategies, query optimization, normalization, and hardware 
considerations. For vector databases, optimization often involves choosing appropriate similarity metrics, 
implementing efficient indexing algorithms like HNSW (Hierarchical Navigable Small World), and balancing between 
search speed and accuracy through techniques like quantization.`,
		},
		{
			Title: "Open Source Software Development",
			Content: `Open source software development is a decentralized software development model that encourages 
open collaboration. The main principle of open source software development is peer production, where products are 
developed by independent contributors rather than by a single company. Open source projects benefit from diverse 
contributions, transparency, and community-driven innovation. Popular examples include Linux, Apache, and countless 
programming libraries and frameworks.`,
		},
		{
			Title: "Cloud Computing Architecture",
			Content: `Cloud computing architecture refers to the components and subcomponents required for cloud computing. 
These components typically consist of a front end platform, back end platforms, a cloud-based delivery, and a network. 
Combined, these components make up cloud computing architecture. Cloud computing has revolutionized how applications 
are deployed and scaled, offering benefits like elasticity, cost efficiency, and global accessibility. Major cloud 
providers include AWS, Google Cloud, and Microsoft Azure.`,
		},
	}
}